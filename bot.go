// IRC bot written mostly for handling URLs posted on the channels.
package papaBot

import (
	"database/sql"
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/fatih/color"
	_ "github.com/mattn/go-sqlite3"
	"github.com/nickvanw/ircx"
	"github.com/sorcix/irc"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"strings"
	"text/template"
	"time"
)

const Version = "0.7.0"

// New creates a new bot.
func New(configFile, textsFile string) *Bot {
	rand.Seed(time.Now().Unix())

	r := color.New(color.FgHiRed).SprintfFunc()
	y := color.New(color.FgHiYellow).SprintfFunc()
	p := color.New(color.FgHiMagenta).SprintfFunc()
	// Init bot struct
	bot := &Bot{
		floodSemaphore: make(chan int, 5),
		logDebug:       log.New(os.Stdout, p("DEBUG: "), log.Ltime),
		logInfo:        log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime),
		logWarn:        log.New(os.Stdout, y("WARN: "), log.Ldate|log.Ltime|log.Lshortfile),
		logError:       log.New(os.Stderr, r("ERROR: "), log.Ldate|log.Ltime|log.Lshortfile),
		kickedFrom:     map[string]bool{},
		configFile:     configFile,
		textsFile:      textsFile,

		Config: Configuration{
			AntiFloodDelay: 5,
			LogChannel:     true,
			CommandsPer5:   3,
		},

		Texts: botTexts{},
	}
	bot.logInfo.Println("I am papaBot, version", Version)

	// Load config
	if err := bot.loadConfig(); err != nil {
		bot.logError.Fatal("Can't load config:", err)
	}

	// Load texts
	if err := bot.loadTexts(); err != nil {
		bot.logError.Fatal("Can't load texts:", err)
	}

	// Init database
	if err := bot.initDb(); err != nil {
		bot.logError.Fatal("Can't init database:", err)
	}

	// Init underlying irc bot
	bot.irc = ircx.Classic(bot.Config.Server, bot.Config.Name)
	bot.irc.Config.User = bot.Config.User
	bot.irc.Config.MaxRetries = 9999

	// Attach event handlers
	bot.attachEventHandlers()

	// Init bot commands
	bot.initBotCommands()

	// Create log folder
	if bot.Config.LogChannel {
		exists, err := DirExists("logs")
		if err != nil {
			bot.logError.Fatal("Can't check if logs dir exists:", err)
		}
		if !exists {
			if err := os.Mkdir("logs", 0700); err != nil {
				bot.logError.Fatal("Can't create logs folder:", err)
			}
		}
	}

	// Init processors
	if err := initUrlProcessor(bot); err != nil {
		bot.logError.Fatal("Can't init URL processor:", err)
	}

	return bot
}

// loadConfig loads JSON configuration for the bot.
func (bot *Bot) loadConfig() error {
	// Load raw config file
	configFile, err := ioutil.ReadFile(bot.configFile)
	if err != nil {
		bot.logError.Fatalln("Can't load config file:", err)
	}
	// Decode TOML
	if _, err := toml.Decode(string(configFile), &bot.Config); err != nil {
		return err
	}

	// Bot owners password
	if bot.Config.OwnerPassword == "" { // Don't allow empty passwords
		bot.logError.Fatal("You must set OwnerPassword in your config.")
	}
	if !strings.HasPrefix(bot.Config.OwnerPassword, "hash:") { // Password needs to be hashed
		bot.logInfo.Println("Pasword not hashed. Hashing and saving.")
		bot.Config.OwnerPassword = HashPassword(bot.Config.OwnerPassword)
		// Now rewrite the password line in the config
		lines := strings.Split(string(configFile), "\n")

		for i, line := range lines {
			if strings.HasPrefix(strings.Trim(line, " \t"), "ownerpassword") {
				lines[i] = fmt.Sprintf("ownerpassword = \"%s\"", bot.Config.OwnerPassword)
			}
		}
		output := strings.Join(lines, "\n")
		err = ioutil.WriteFile(bot.configFile, []byte(output), 0644)
		if err != nil {
			bot.logError.Fatalln("Can't save config:", err)
		}
	}

	return nil
}

// loadTexts loads bot texts from file.
func (bot *Bot) loadTexts() error {
	// Decode TOML
	if _, err := toml.DecodeFile(bot.textsFile, &bot.Texts); err != nil {
		return err
	}
	// Parse the templates
	temp, err := template.New("tpl1").Parse(bot.Texts.DuplicateFirst)
	if err != nil {
		bot.logError.Fatalln("Error in the text '", bot.Texts.DuplicateFirst, "': ", err)
	} else {
		bot.Texts.tempDuplicateFirst = temp
	}
	temp, err = template.New("tpl2").Parse(bot.Texts.DuplicateMulti)
	if err != nil {
		bot.logError.Fatalln("Error in the text '", bot.Texts.DuplicateMulti, "': ", err)
	} else {
		bot.Texts.tempDuplicateMulti = temp
	}
	return nil
}

// initDb initializes the bot's database.
func (bot *Bot) initDb() error {
	db, err := sql.Open("sqlite3", "papabot.db")
	if err != nil {
		return err
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
	bot.floodSemaphore <- 1
	bot.irc.Sender.Send(&irc.Message{
		Command:  mType,
		Params:   []string{channel},
		Trailing: message,
	})
}

// SendMessage sends a message to the channel.
func (bot *Bot) SendMessage(channel, message string) {
	bot.sendFloodProtected(irc.PRIVMSG, channel, message)
}

// SendNotice send a notice to the channel.
func (bot *Bot) SendNotice(channel, message string) {
	bot.sendFloodProtected(irc.NOTICE, channel, message)

}

// isMe checks if the sender is the bot.
func (bot *Bot) isMe(name string) bool {
	return name == bot.irc.OriginalName
}

// areSamePeople checks if two nicks belong to the same person.
func (bot *Bot) areSamePeople(nick1, nick2 string) bool {
	nick1 = strings.Trim(nick1, "_~")
	nick2 = strings.Trim(nick2, "_~")
	return nick1 == nick2
}

// Scribe saves the message into appropriate file.
func (bot *Bot) Scribe(channel string, message ...interface{}) {
	if !bot.Config.LogChannel {
		return
	}
	go func() {
		logFileName := fmt.Sprintf("logs/%s_%s.txt", channel, time.Now().Format("2006-01-02"))
		f, err := os.OpenFile(logFileName, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			bot.logError.Println("Error opening log file:", err)
			return
		}
		defer f.Close()

		scribe := log.New(f, "", log.Ldate|log.Ltime)
		scribe.Println(message...)
	}()
}

// Close cleans up after the bot.
func (bot *Bot) Close() {
	bot.Db.Close()
}

// Run starts the bot's main loop.
func (bot *Bot) Run() {
	defer bot.Close()

	// Connect to server.
	bot.logInfo.Println("Connecting to", bot.Config.Server, "...")
	if err := bot.irc.Connect(); err != nil {
		bot.logError.Fatal("Error creating connection:", err)
	}

	// Semaphore clearing ticker.
	ticker := time.NewTicker(time.Second * time.Duration(bot.Config.AntiFloodDelay))
	defer ticker.Stop()
	go func() {
		for range ticker.C {
			bot.resetFloodSemaphore()
		}
	}()

	// Command use clearing ticker.
	ticker2 := time.NewTicker(time.Minute * 5)
	defer ticker2.Stop()
	go func() {
		for range ticker2.C {
			for k := range bot.commandUseLimit {
				delete(bot.commandUseLimit, k)
			}
		}
	}()

	// Main loop.
	bot.irc.HandleLoop()
}
