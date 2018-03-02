// Package papaBot provides an IRC bot with focus on easy extension and customization.
package papaBot

import (
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/onrik/logrus/filename"
	"github.com/pawelszydlo/papa-bot/events"
	"github.com/pawelszydlo/papa-bot/transports"
	"github.com/pawelszydlo/papa-bot/transports/irc"
	"github.com/pawelszydlo/papa-bot/utils"
	"github.com/pelletier/go-toml"
	"github.com/sirupsen/logrus"
	"regexp"
	"strings"
	"sync"
)

const (
	Version = "0.11"
	Debug   = false // Set to true to crash on runtime errors.
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
		transports: map[string]transports.Transport{},
	}
	// Logging configuration.
	bot.Log.Level = bot.Config.LogLevel
	bot.Log.Formatter = &logrus.TextFormatter{FullTimestamp: true, TimestampFormat: "2006-01-02][15:04:05"}
	filenameHook := filename.NewHook()
	filenameHook.Field = "source"
	bot.Log.AddHook(filenameHook)

	// Setup event dispatcher.
	bot.EventDispatcher = events.New(bot.Log)

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

	// Init the ignore list.
	ignored := strings.Split(bot.GetVar("_ignored"), " ")
	bot.EventDispatcher.SetBlackList(ignored)
	bot.Log.Infof("Ignoring users: %s", strings.Join(ignored, ", "))

	// Init bot commands.
	bot.initBotCommands()

	// Attach event listeners.
	bot.attachEventListeners()

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

// attachEventListeners will attach all built-in listeners.
func (bot *Bot) attachEventListeners() {
	// Logging.
	bot.EventDispatcher.RegisterMultiListener(events.EventsChannelActivity, bot.scribeListener)
	bot.EventDispatcher.RegisterMultiListener(events.EventsChannelMessages, bot.scribeListener)
	// Messages.
	bot.EventDispatcher.RegisterListener(events.EventChatMessage, bot.messageListener)
	bot.EventDispatcher.RegisterListener(events.EventPrivateMessage, bot.messageListener)
	// URLs.
	bot.EventDispatcher.RegisterListener(events.EventChatMessage, bot.handleURLsListener)
	bot.EventDispatcher.RegisterListener(events.EventPrivateMessage, bot.handleURLsListener)
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

func (bot *Bot) getTransportOrDie(name string) transports.Transport {
	if transport, ok := bot.transports[name]; ok {
		return transport
	}
	bot.Log.Panicf("Code wanted transport %s, but it doesn't exist.", name)
	return nil
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

	// Create a wait group.
	wait := sync.WaitGroup{}

	// Start transports.
	bot.Log.Info("Starting transports...")
	for _, transport := range bot.transports {
		wait.Add(1)
		go func() {
			defer wait.Done()
			transport.Run()
		}()
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
			// Check if it's time for a daily ticker.
			if time.Since(bot.nextDailyTick) >= 0 {
				bot.nextDailyTick = bot.nextDailyTick.Add(24 * time.Hour)
				bot.Log.Debugf("Daily tick now. Next at %s.", bot.nextDailyTick)
				bot.EventDispatcher.Trigger(events.EventMessage{"bot", events.EventDailyTick, "", "", "", "", true})
			} else {
				bot.EventDispatcher.Trigger(events.EventMessage{"bot", events.EventTick, "", "", "", "", true})
			}
		}
	}()
	// First tick, before ticker goes off.
	bot.EventDispatcher.Trigger(events.EventMessage{"bot", events.EventTick, "", "", "", "", true})

	// Wait for all the transports to finish.
	wait.Wait()

	bot.Log.Infof("Exiting...")
}
