// Package papaBot provides an IRC bot with focus on easy extension and customization.
package papaBot

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/pawelszydlo/papa-bot/transports/irc"
	"github.com/pawelszydlo/papa-bot/utils"
	"github.com/pelletier/go-toml"
	"regexp"
	"strings"
)

const (
	Version           = "0.10"
	Debug             = false // Set to true to crash on runtime errors.
	MessageBufferSize = 10
)

// New creates a new bot.
func New(configFile, textsFile string) *Bot {
	rand.Seed(time.Now().Unix())

	// Load config file.
	fullConfig, err := toml.LoadFile(configFile)
	if err != nil {
		log.Fatalf("Can't load config: %s", err)
	}
	// Load texts file.
	fullTexts, err := toml.LoadFile(textsFile)
	if err != nil {
		log.Fatalf("Can't load texts: %s", err)
	}

	// Prepare configuration.
	config := Configuration{
		ChatLogging:                fullConfig.GetDefault("bot.chat_logging", true).(bool),
		UrlAnnounceIntervalMinutes: time.Duration(fullConfig.GetDefault("bot.url_announce_interval_minutes", 15).(int64)),
		CommandsPer5:               10,
		UrlAnnounceIntervalLines:   int(fullConfig.GetDefault("bot.url_announce_interval_lines", 50).(int64)),
		PageBodyMaxSize:            100 * 1024,
		HttpDefaultUserAgent:       fullConfig.GetDefault("bot.http_user_agent", "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)").(string),
		DailyTickHour:              int(fullConfig.GetDefault("bot.daily_tick_hour", 8).(int64)),
		DailyTickMinute:            0,
		Language:                   fullConfig.GetDefault("bot.language", "en").(string),
		Name:                       fullConfig.GetDefault("bot.name", "papaBot").(string),
		LogLevel:                   logrus.DebugLevel,
	}

	// Init bot struct.
	bot := &Bot{
		initDone:            false,
		Log:                 logrus.New(),
		HTTPClient:          &http.Client{Timeout: 10 * time.Second},
		authenticatedUsers:  map[string]string{},
		authenticatedAdmins: map[string]string{},
		authenticatedOwners: map[string]string{},

		fullTexts: fullTexts,
		Texts:     &botTexts{},

		lastURLAnnouncedTime:        map[string]time.Time{},
		lastURLAnnouncedLinesPassed: map[string]int{},
		urlMoreInfo:                 map[string]string{},

		fullConfig: fullConfig,
		Config:     &config,

		commands:           map[string]*BotCommand{},
		commandUseLimit:    map[string]int{},
		commandWarn:        map[string]bool{},
		commandsHideParams: map[string]bool{},

		customVars:         map[string]string{},
		webContentSampleRe: regexp.MustCompile(`(?i)<[^>]*?description[^<]*?>|<title>.*?</title>`),

		extensions: []extension{},
		transports: map[string]transportWrapper{},
	}
	// Logging configuration.
	bot.Log.Level = bot.Config.LogLevel
	bot.Log.Formatter = &logrus.TextFormatter{FullTimestamp: true, TimestampFormat: "2006-01-02][15:04:05"}

	// Register built-in transports.
	bot.RegisterTransport("irc", ircTransport.New)

	// Load texts.
	if err := bot.LoadTexts("bot", bot.Texts); err != nil {
		bot.Log.Fatalf("Can't load bot texts: %s", err)
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

	// Init extensions.
	for i := range bot.extensions {
		if err := bot.extensions[i].Init(bot); err != nil {
			bot.Log.Fatalf("Error loading extensions: %s", err)
		}
	}

	bot.initDone = true
	bot.Log.Infof("Bot init done.")
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

func (bot *Bot) getTransportWrapOrDie(name string) *transportWrapper {
	if wrap, ok := bot.transports[name]; ok {
		return &wrap
	}
	bot.Log.Panicf("Code wanted wrapper for transport %s, but it doesn't exist.", name)
	return nil
}

// isNickIgnored will check whether given nick is ignored.
func (bot *Bot) isNickIgnored(transport, nick string) bool {
	// Custom vars are held in memory so this should be fast enough.
	ignored := strings.Split(bot.GetVar("_ignore"), " ")
	for _, person := range ignored {
		if person == transport+"~"+nick {
			return true
		}
	}
	return false
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

// cleanUp cleans up after the bot.
func (bot *Bot) cleanUp() {
	bot.Db.Close()
}

// Run starts the bot's main loop.
func (bot *Bot) Run() {
	// Initialize bot mechanisms.
	bot.initialize()
	defer bot.cleanUp()

	// Start transports.
	bot.Log.Info("Starting transports...")
	for _, wrap := range bot.transports {
		go wrap.transport.Run()
	}

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
		for transportName, transportWrapper := range bot.transports {
			select {
			// Check for scribe messages.
			case message, ok := <-transportWrapper.scribeChannel:
				if ok {
					bot.handleMessage(transportName, message)
				} else {
					break
				}
			// Check for command messages.
			case message, ok := <-transportWrapper.commandsChannel:
				if ok {
					bot.handleBotCommand(transportName, message)
				} else {
					break
				}
			}
		}
	}

	bot.Log.Infof("Exiting...")
}
