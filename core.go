// Package papaBot provides an IRC bot with focus on easy extension and customization.
package papaBot

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"crypto/tls"
	"net"

	"github.com/BurntSushi/toml"
	"github.com/Sirupsen/logrus"
	"github.com/pawelszydlo/papa-bot/lexical"
	"github.com/pawelszydlo/papa-bot/utils"
	"github.com/sorcix/irc"
	"regexp"
)

const (
	Version        = "0.9.9.2"
	Debug          = false // Set to true to crash on runtime errors.
	MsgLengthLimit = 440   // IRC message length limit.
)

// New creates a new bot.
func New(configFile, textsFile string) *Bot {
	rand.Seed(time.Now().Unix())

	// Default configuration.
	config := Configuration{
		AntiFloodDelay:             5,
		ChatLogging:                true,
		CommandsPer5:               3,
		UrlAnnounceIntervalMinutes: 15,
		UrlAnnounceIntervalLines:   50,
		RejoinDelay:                15 * time.Second,
		PageBodyMaxSize:            100 * 1024,
		HttpDefaultUserAgent:       "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
		DailyTickHour:              8,
		DailyTickMinute:            0,
		Language:                   "en",
		Name:                       "papaBot",
		User:                       "papaBot",
		LogLevel:                   logrus.DebugLevel,
	}

	// Init bot struct.
	bot := &Bot{
		initDone:            false,
		irc:                 &ircConnection{messages: make(chan *irc.Message)},
		Log:                 logrus.New(),
		HTTPClient:          &http.Client{Timeout: 10 * time.Second},
		floodSemaphore:      make(chan int, 5),
		kickedFrom:          map[string]bool{},
		onChannel:           map[string]bool{},
		authenticatedUsers:  map[string]string{},
		authenticatedAdmins: map[string]string{},
		authenticatedOwners: map[string]string{},

		TextsFile: textsFile,
		Texts:     &botTexts{},

		lastURLAnnouncedTime:        map[string]time.Time{},
		lastURLAnnouncedLinesPassed: map[string]int{},
		urlMoreInfo:                 map[string]string{},

		ConfigFile: configFile,
		Config:     &config,

		ircEventHandlers:   make(map[string][]ircEvenHandlerFunc),
		commands:           map[string]*BotCommand{},
		commandUseLimit:    map[string]int{},
		commandWarn:        map[string]bool{},
		commandsHideParams: map[string]bool{},

		customVars:         map[string]string{},
		webContentSampleRe: regexp.MustCompile(`(?i)<[^>]*?description[^<]*?>|<title>.*?</title>`),

		extensions: []extension{},
	}
	// Logging configuration.
	bot.Log.Level = bot.Config.LogLevel
	bot.Log.Formatter = &logrus.TextFormatter{FullTimestamp: true, TimestampFormat: "2006-01-02][15:04:05"}

	// Load config.
	if _, err := toml.DecodeFile(bot.ConfigFile, &bot.Config); err != nil {
		bot.Log.Fatalf("Can't load config: %s", err)
	}

	// Load texts.
	if err := bot.LoadTexts(bot.TextsFile, bot.Texts); err != nil {
		bot.Log.Fatalf("Can't load texts: %s", err)
	}

	return bot
}

// initialize performs initialization of bot's mechanisms.
func (bot *Bot) initialize() {
	bot.Log.Infof("I am papaBot, version %s", Version)

	// Init database.
	if err := bot.initDb(); err != nil {
		bot.Log.Fatalf("Can't init database: %s", err)
	}
	bot.ensureOwnerExists()

	// Create log folder.
	if bot.Config.ChatLogging {
		exists, err := utils.DirExists("logs")
		if err != nil {
			bot.Log.Fatalf("Can't check if logs dir exists: %s", err)
		}
		if !exists {
			if err := os.Mkdir("logs", 0700); err != nil {
				bot.Log.Fatalf("Can't create logs folder: %s", err)
			}
		}
	}

	// Load custom vars.
	bot.loadVars()

	// Attach event handlers.
	bot.assignEventHandlers()

	// Init bot commands.
	bot.initBotCommands()

	// Get next daily tick.
	now := time.Now()
	bot.nextDailyTick = time.Date(
		now.Year(), now.Month(), now.Day(), bot.Config.DailyTickHour, bot.Config.DailyTickMinute, 0, 0, now.Location())
	if time.Since(bot.nextDailyTick) >= 0 {
		bot.nextDailyTick = bot.nextDailyTick.Add(24 * time.Hour)
	}
	bot.Log.Debugf("Next daily tick: %s", bot.nextDailyTick)

	// Load stopwords.
	if err := lexical.LoadStopWords(bot.Config.Language); err != nil {
		bot.Log.Warningf("Can't load stop words for language %s: %s", bot.Config.Language, err)
		bot.Log.Warningf("Lexical functions will be handicapped.")
	}
	bot.Log.Debugf("Loaded %d stopwords for language %s.", len(lexical.StopWords), bot.Config.Language)

	// Init extensions.
	for i := range bot.extensions {
		if err := bot.extensions[i].Init(bot); err != nil {
			bot.Log.Fatalf("Error loading extensions: %s", err)
		}
	}

	bot.initDone = true
	bot.Log.Infof("Bot init done.")
}

// connect attempts to connect to the given IRC server.
func (bot *Bot) connect() error {
	var conn net.Conn
	var err error
	// Establish the connection.
	if bot.Config.TLSConfig == nil {
		bot.Log.Infof("Connecting to %s...", bot.Config.Server)
		conn, err = net.Dial("tcp", bot.Config.Server)
	} else {
		bot.Log.Infof("Connecting to %s using TLS...", bot.Config.Server)
		conn, err = tls.Dial("tcp", bot.Config.Server, bot.Config.TLSConfig)
	}
	if err != nil {
		return err
	}

	// Store connection.
	bot.irc.connection = conn
	bot.irc.decoder = irc.NewDecoder(conn)
	bot.irc.encoder = irc.NewEncoder(conn)

	// Send initial messages.
	if bot.Config.Password != "" {
		bot.SendRawMessage(irc.PASS, []string{bot.Config.Password}, "")
	}
	bot.SendRawMessage(irc.NICK, []string{bot.Config.Name}, "")
	bot.SendRawMessage(irc.USER, []string{bot.Config.User, "0", "*"}, bot.Config.User)

	// Run the message receiver loop.
	go bot.receiverLoop()

	bot.Log.Debugf("Succesfully connected.")
	return nil
}

// receiverLoop attempts to read from the IRC server and keep the connection open.
func (bot *Bot) receiverLoop() {
	for {
		bot.irc.connection.SetDeadline(time.Now().Add(300 * time.Second))
		msg, err := bot.irc.decoder.Decode()
		if err != nil { // Error or timeout.
			bot.Log.Warningf("Disconnected from server.")
			bot.irc.connection.Close()
			retries := 0
			for {
				time.Sleep(time.Duration(retries*retries) * time.Second)
				bot.Log.Infof("Reconnecting...")
				if err := bot.connect(); err == nil {
					break
				}
				retries += 1
			}
		} else {
			bot.irc.messages <- msg
		}
	}
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

// sendFloodProtected is a flood protected message sender.
func (bot *Bot) sendFloodProtected(mType, channel, message string) {
	messages := strings.Split(message, "\n")
	for i := range messages {
		// IRC message size limit.
		if len(messages[i]) > MsgLengthLimit {
			for n := 0; n < len(messages[i]); n += MsgLengthLimit {
				upperLimit := n + MsgLengthLimit
				if upperLimit > len(messages[i]) {
					upperLimit = len(messages[i])
				}
				bot.floodSemaphore <- 1
				bot.SendRawMessage(mType, []string{channel}, messages[i][n:upperLimit])
			}
			return
		}
		bot.floodSemaphore <- 1
		bot.SendRawMessage(mType, []string{channel}, messages[i])
	}
}

// loadVars loads all custom variables from the database.
func (bot *Bot) loadVars() {
	result, err := bot.Db.Query(`SELECT name, value FROM vars`)
	if err != nil {
		return
	}
	defer result.Close()

	// Get vars.
	for result.Next() {
		var name string
		var value string
		if err = result.Scan(&name, &value); err != nil {
			bot.Log.Warningf("Can't load var: %s", err)
			continue
		}
		bot.customVars[name] = value
	}
}

// scribe saves the message into appropriate channel log file.
func (bot *Bot) scribe(channel string, message ...interface{}) {
	if !bot.Config.ChatLogging {
		return
	}
	go func() {
		logFileName := fmt.Sprintf("logs/%s_%s.txt", channel, time.Now().Format("2006-01-02"))
		f, err := os.OpenFile(logFileName, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			bot.Log.Errorf("Error opening log file: %s", err)
			return
		}
		defer f.Close()

		scribe := log.New(f, "", log.Ldate|log.Ltime)
		scribe.Println(message...)
	}()
}

// runExtensionTickers will asynchronously run all extension tickers.
func (bot *Bot) runExtensionTickers() {
	currentExtension := ""
	// Catch errors.
	defer func() {
		if Debug {
			return
		} // When in debug mode fail on all errors.
		if r := recover(); r != nil {
			bot.Log.WithField("ext", currentExtension).Errorf("FATAL ERROR in tickers: %s", r)
		}
	}()

	// Check if it's time for a daily ticker.
	daily := false
	if time.Since(bot.nextDailyTick) >= 0 {
		daily = true
		bot.nextDailyTick = bot.nextDailyTick.Add(24 * time.Hour)
		bot.Log.Debugf("Daily tick now. Next at %s.", bot.nextDailyTick)
	}

	// Run the tickers.
	for i := range bot.extensions {
		currentExtension = fmt.Sprintf("%T", bot.extensions[i])
		bot.extensions[i].Tick(bot, daily)
	}
}

// close cleans up after the bot.
func (bot *Bot) close() {
	bot.Db.Close()
	close(bot.irc.messages)
	close(bot.floodSemaphore)
}

// Run starts the bot's main loop.
func (bot *Bot) Run() {
	// Initialize bot mechanisms.
	bot.initialize()
	defer bot.close()

	// Connect to server.
	if err := bot.connect(); err != nil {
		bot.Log.Fatalf("Error creating connection: ", err)
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
	for {
		select {
		case msg, ok := <-bot.irc.messages:
			if ok {
				// Are there any handlers registered for this IRC event?
				if handlers, exists := bot.ircEventHandlers[msg.Command]; exists {
					for _, handler := range handlers {
						handler(bot, msg)
					}
				}
			} else {
				break
			}
		}
	}

	bot.Log.Infof("Exiting...")
}
