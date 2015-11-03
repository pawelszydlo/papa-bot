// Package papaBot provides an easily extensible IRC bot written mostly for handling URLs posted on the channels.
package papaBot

import (
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"reflect"
	"strings"
	"text/template"
	"time"

	"errors"

	"github.com/BurntSushi/toml"
	_ "github.com/mattn/go-sqlite3"
	"github.com/nickvanw/ircx"
	"github.com/op/go-logging"
	"github.com/sorcix/irc"
	"golang.org/x/crypto/pbkdf2"
	"golang.org/x/net/html/charset"
	"golang.org/x/text/transform"
)

const (
	Version = "0.8.2"
)

// New creates a new bot.
func New(configFile, textsFile string) *Bot {
	rand.Seed(time.Now().Unix())

	// Init bot struct.
	bot := &Bot{
		log:            logging.MustGetLogger("bot"),
		HTTPClient:     &http.Client{Timeout: 5 * time.Second},
		floodSemaphore: make(chan int, 5),
		kickedFrom:     map[string]bool{},

		textsFile: textsFile,
		Texts:     &BotTexts{},

		lastURLAnnouncedTime:        map[string]time.Time{},
		lastURLAnnouncedLinesPassed: map[string]int{},
		urlMoreInfo:                 map[string]string{},

		configFile: configFile,
		Config: Configuration{
			AntiFloodDelay:             5,
			LogChannel:                 true,
			CommandsPer5:               3,
			UrlAnnounceIntervalMinutes: 15,
			UrlAnnounceIntervalLines:   50,
			RejoinDelaySeconds:         15,
			PageBodyMaxSize:            50 * 1024,
			HttpUserAgent:              "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
		},

		commands:        map[string]*BotCommand{},
		commandUseLimit: map[string]int{},
		commandWarn:     map[string]bool{},

		// Register extensions (ordering matters!)
		extensions: []Extension{
			new(ExtensionMeta),
			new(ExtensionDuplicates),
			new(ExtensionGitHub),
			new(ExtensionBtc),
			new(ExtensionReddit),
		},
	}
	// Logging init.
	formatNorm := logging.MustStringFormatter(
		"%{color}[%{time:2006/01/02 15:04:05}] %{level:.4s} ▶%{color:reset} %{message}",
	)
	backendNorm := logging.NewLogBackend(os.Stdout, "", 0)
	backendNormFormatted := logging.NewBackendFormatter(backendNorm, formatNorm)
	logging.SetBackend(backendNormFormatted)

	bot.log.Info("I am papaBot, version %s", Version)

	// Load config.
	if err := bot.loadConfig(); err != nil {
		bot.log.Error("Can't load config: %s", err)
	}

	// Load texts.
	if err := bot.LoadTexts(bot.textsFile, bot.Texts); err != nil {
		bot.log.Fatal("Can't load texts: %s", err)
	}

	// Init database.
	if err := bot.initDb(); err != nil {
		bot.log.Fatal("Can't init database: %s", err)
	}

	// Create log folder.
	if bot.Config.LogChannel {
		exists, err := DirExists("logs")
		if err != nil {
			bot.log.Fatal("Can't check if logs dir exists: %s", err)
		}
		if !exists {
			if err := os.Mkdir("logs", 0700); err != nil {
				bot.log.Fatal("Can't create logs folder: %s", err)
			}
		}
	}

	// Init underlying irc bot.
	bot.irc = ircx.Classic(bot.Config.Server, bot.Config.Name)
	bot.irc.Config.User = bot.Config.User
	bot.irc.Config.MaxRetries = 9999

	// Attach event handlers.
	bot.attachEventHandlers()

	// Init bot commands.
	bot.initBotCommands()

	// Init extensions.
	for i := range bot.extensions {
		if err := bot.extensions[i].Init(bot); err != nil {
			bot.log.Fatal("Error loading extensions: %s", err)
		}
	}

	return bot
}

// loadConfig loads JSON configuration for the bot.
func (bot *Bot) loadConfig() error {

	// Load raw config file.
	configFile, err := ioutil.ReadFile(bot.configFile)
	if err != nil {
		bot.log.Fatal("Can't load config file: %s", err)
	}

	// Decode TOML.
	if _, err := toml.Decode(string(configFile), &bot.Config); err != nil {
		return err
	}

	// Bot owners password.
	if bot.Config.OwnerPassword == "" { // Don't allow empty passwords.
		bot.log.Fatal("You must set OwnerPassword in your config.")
	}
	if !strings.HasPrefix(bot.Config.OwnerPassword, "hash:") { // Password needs to be hashed.
		bot.log.Info("Pasword not hashed. Hashing and saving.")
		bot.Config.OwnerPassword = bot.HashPassword(bot.Config.OwnerPassword)
		// Now rewrite the password line in the config.
		lines := strings.Split(string(configFile), "\n")

		for i, line := range lines {
			if strings.HasPrefix(strings.Trim(line, " \t"), "ownerpassword") {
				lines[i] = fmt.Sprintf("ownerpassword = \"%s\"", bot.Config.OwnerPassword)
			}
		}
		output := strings.Join(lines, "\n")
		err = ioutil.WriteFile(bot.configFile, []byte(output), 0644)
		if err != nil {
			bot.log.Fatal("Can't save config: %s", err)
		}
	}

	return nil
}

// initDb initializes the bot's database.
func (bot *Bot) initDb() error {
	db, err := sql.Open("sqlite3", "papabot.db")
	if err != nil {
		return err
	}

	// Create URLs tables and triggers, if needed.
	query := `
		CREATE TABLE IF NOT EXISTS "urls" (
			"id" INTEGER PRIMARY KEY  AUTOINCREMENT  NOT NULL,
			"channel" VARCHAR NOT NULL,
			"nick" VARCHAR NOT NULL,
			"link" VARCHAR NOT NULL,
			"quote" VARCHAR NOT NULL,
			"title" VARCHAR,
			"timestamp" DATETIME DEFAULT (datetime('now','localtime'))
		);

		CREATE VIRTUAL TABLE IF NOT EXISTS urls_search
		USING fts4(channel, nick, link, title, timestamp, search);

		CREATE TRIGGER IF NOT EXISTS url_add AFTER INSERT ON urls BEGIN
			INSERT INTO urls_search(channel, nick, link, title, timestamp, search)
			VALUES(new.channel, new.nick, new.link, new.title, new.timestamp, new.link || ' ' || new.title);
		END;

		CREATE TRIGGER IF NOT EXISTS url_update AFTER UPDATE ON urls BEGIN
			UPDATE urls_search SET title = new.title, search = new.link || ' ' || new.title
			WHERE timestamp = new.timestamp;
		END;`
	if _, err := db.Exec(query); err != nil {
		bot.log.Panic(err)
	}

	bot.Db = db
	return nil
}

// resetFloodSemaphore flushes bot's flood semaphore.
func (bot *Bot) resetFloodSemaphore() {
	for {
		select {
		case <-bot.floodSemaphore:
			continue
		default:
			return
		}
	}
}

// sendFloodProtected is a flood protected sender.
func (bot *Bot) sendFloodProtected(mType, channel, message string) {
	messages := strings.Split(message, "\n")
	for i := range messages {
		bot.floodSemaphore <- 1
		bot.irc.Sender.Send(&irc.Message{
			Command:  mType,
			Params:   []string{channel},
			Trailing: messages[i],
		})
	}

}

// SendMessage sends a message to the channel.
func (bot *Bot) SendMessage(channel, message string) {
	bot.log.Debug("Sending message to %s: %s", channel, message)
	bot.sendFloodProtected(irc.PRIVMSG, channel, message)
}

// SendNotice sends a notice to the channel.
func (bot *Bot) SendNotice(channel, message string) {
	bot.log.Debug("Sending notice to %s: %s", channel, message)
	bot.sendFloodProtected(irc.NOTICE, channel, message)
}

// SendMassNotice sends a notice to all the channels bot is on.
func (bot *Bot) SendMassNotice(message string) {
	for _, channel := range bot.Config.Channels {
		bot.sendFloodProtected(irc.NOTICE, channel, message)
	}
}

// isMe checks if the sender is the bot.
func (bot *Bot) IsMe(name string) bool {
	return name == bot.irc.OriginalName
}

// areSamePeople checks if two nicks belong to the same person.
func (bot *Bot) AreSamePeople(nick1, nick2 string) bool {
	nick1 = strings.Trim(nick1, "_~")
	nick2 = strings.Trim(nick2, "_~")
	return nick1 == nick2
}

// scribe saves the message into appropriate channel log file.
func (bot *Bot) Scribe(channel string, message ...interface{}) {
	if !bot.Config.LogChannel {
		return
	}
	go func() {
		logFileName := fmt.Sprintf("logs/%s_%s.txt", channel, time.Now().Format("2006-01-02"))
		f, err := os.OpenFile(logFileName, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			bot.log.Error("Error opening log file: %s", err)
			return
		}
		defer f.Close()

		scribe := log.New(f, "", log.Ldate|log.Ltime)
		scribe.Println(message...)
	}()
}

// GetPageBodyByURL is a convenience wrapper around GetPageBody.
func (bot *Bot) GetPageBodyByURL(url string) ([]byte, error) {
	urlinfo := &UrlInfo{url, "", "", []byte{}, "", ""}
	if err := bot.GetPageBody(urlinfo); err != nil {
		return urlinfo.Body, err
	}
	return urlinfo.Body, nil
}

// GetPageBody gets and returns a body of a page.
func (bot *Bot) GetPageBody(urlinfo *UrlInfo) error {
	if urlinfo.URL == "" {
		return errors.New("Empty URL")
	}
	// Build the request.
	req, err := http.NewRequest("GET", urlinfo.URL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", bot.Config.HttpUserAgent)

	// Get response.
	resp, err := bot.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Update the URL if it changed after redirects.
	final_link := resp.Request.URL.String()
	if final_link != "" && final_link != urlinfo.URL {
		bot.log.Debug("%s becomes %s", urlinfo.URL, final_link)
		urlinfo.URL = final_link
	}

	// Load the body up to PageBodyMaxSize.
	body := make([]byte, bot.Config.PageBodyMaxSize, bot.Config.PageBodyMaxSize)
	if num, err := io.ReadFull(resp.Body, body); err != nil && err != io.ErrUnexpectedEOF {
		return err
	} else {
		// Trim unneeded 0 bytes so that JSON unmarshaller won't complain.
		body = body[:num]
	}
	// Get the content-type
	content_type := resp.Header.Get("Content-Type")
	if content_type == "" {
		content_type = http.DetectContentType(body)
	}
	urlinfo.ContentType = content_type

	// If type is text, decode the body to UTF-8.
	if strings.Contains(content_type, "text/html") || strings.Contains(content_type, "text/plain") {
		encoding, _, _ := charset.DetermineEncoding(body, content_type)
		decodedBody, _, _ := transform.Bytes(encoding.NewDecoder(), body)
		urlinfo.Body = decodedBody
	} else if strings.Contains(content_type, "application/json") {
		urlinfo.Body = body
	} else {
		bot.log.Debug("Not fetching the body for Content-Type: %s", content_type)
	}
	return nil
}

// LoadTexts loads texts from a file into a struct, auto handling the templates.
func (bot *Bot) LoadTexts(filename string, data interface{}) error {

	// Decode TOML
	if _, err := toml.DecodeFile(filename, data); err != nil {
		return err
	}

	// Fields starting with "Tpl" with be parsed into templates and saved in the field starting with "Temp".
	rData := reflect.ValueOf(data).Elem()
	missingTexts := false
	for i := 0; i < rData.NumField(); i++ {
		// Get field and it's value.
		field := rData.Type().Field(i)
		fieldValue := rData.Field(i)

		// Check if all fields were filled.
		if !strings.HasPrefix(field.Name, "Temp") {
			if fieldValue.String() == "" {
				bot.log.Warning("Field left empty %s!", field.Name)
				missingTexts = true
			}
		}

		if strings.HasPrefix(field.Name, "Tpl") {
			temp, err := template.New(field.Name).Parse(fieldValue.String())
			if err != nil {
				return err
			} else {
				tempFieldName := strings.TrimPrefix(field.Name, "Tpl")
				tempFieldName = "Temp" + tempFieldName
				// Set template field value.
				tempField := rData.FieldByName(tempFieldName)
				if !tempField.IsValid() {
					bot.log.Fatal("Can't find field %s to store template from %s.", tempFieldName, field.Name)
				}
				if !tempField.CanSet() {
					bot.log.Fatal("Field %s is not settable.", tempFieldName)
				}
				if reflect.ValueOf(temp).Type() != tempField.Type() {
					bot.log.Fatalf("Incompatible types %s and %s", reflect.ValueOf(temp).Type(), tempField.Type())
				}
				tempField.Set(reflect.ValueOf(temp))
			}
		}
	}
	if missingTexts {
		bot.log.Fatal("Missing texts.")
	}

	return nil
}

// HashPassword hashes the password.
func (bot *Bot) HashPassword(password string) string {
	return fmt.Sprintf("hash:%s", base64.StdEncoding.EncodeToString(
		pbkdf2.Key([]byte(password), []byte(password), 4096, sha256.Size, sha256.New)))
}

// runExtensionTickers will asynchronously run all extension tickers.
func (bot *Bot) runExtensionTickers() {
	// Catch errors.
	defer func() {
		if r := recover(); r != nil {
			bot.log.Error("FATAL ERROR in tickers: %s", r)
		}
	}()

	// Run the tickers.
	for i := range bot.extensions {
		bot.extensions[i].Tick(bot, false)
	}
}

// Close cleans up after the bot.
func (bot *Bot) Close() {
	bot.Db.Close()
}

// Run starts the bot's main loop.
func (bot *Bot) Run() {
	defer bot.Close()

	// Connect to server.
	bot.log.Info("Connecting to %s...", bot.Config.Server)
	if err := bot.irc.Connect(); err != nil {
		bot.log.Fatal("Error creating connection: %s", err)
	}

	// Semaphore clearing ticker.
	ticker := time.NewTicker(time.Second * time.Duration(bot.Config.AntiFloodDelay))
	defer ticker.Stop()
	go func() {
		for range ticker.C {
			bot.resetFloodSemaphore()
		}
	}()

	// 5 minute ticker.
	ticker2 := time.NewTicker(time.Minute * 5)
	defer ticker2.Stop()
	go func() {
		for range ticker2.C {
			// Clear command use.
			for k := range bot.commandUseLimit {
				delete(bot.commandUseLimit, k)
			}
			for k := range bot.commandWarn {
				delete(bot.commandWarn, k)
			}
			// Run extension tickers.
			go bot.runExtensionTickers()
		}
	}()
	// First tick, before ticker goes off.
	go bot.runExtensionTickers()

	// Main loop.
	bot.irc.HandleLoop()
}
