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
	"github.com/op/go-logging"
	"github.com/pawelszydlo/papa-bot/lexical"
	"github.com/pawelszydlo/papa-bot/utils"
	"github.com/sorcix/irc"
)

const (
	Version = "0.9.6"
	Debug   = false // Set to true to crash on runtime errors.
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
		ReconnectDelay:             10 * time.Second,
		PageBodyMaxSize:            50 * 1024,
		HttpDefaultUserAgent:       "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
		DailyTickHour:              8,
		DailyTickMinute:            0,
		Language:                   "en",
		Name:                       "papaBot",
		User:                       "papaBot",
	}

	// Init bot struct.
	bot := &Bot{
		initDone:            false,
		irc:                 &ircConnection{messages: make(chan *irc.Message)},
		log:                 logging.MustGetLogger("bot"),
		HTTPClient:          &http.Client{Timeout: 5 * time.Second},
		floodSemaphore:      make(chan int, 5),
		kickedFrom:          map[string]bool{},
		onChannel:           map[string]bool{},
		authenticatedUsers:  map[string]string{},
		authenticatedAdmins: map[string]string{},
		authenticatedOwners: map[string]string{},

		textsFile: textsFile,
		Texts:     &botTexts{},

		lastURLAnnouncedTime:        map[string]time.Time{},
		lastURLAnnouncedLinesPassed: map[string]int{},
		urlMoreInfo:                 map[string]string{},

		configFile: configFile,
		Config:     &config,

		eventHandlers:      map[string]func(msg *irc.Message){},
		commands:           map[string]*BotCommand{},
		commandUseLimit:    map[string]int{},
		commandWarn:        map[string]bool{},
		commandsHideParams: map[string]bool{},

		customVars: map[string]string{},

		// Register built-in extensions (ordering matters!).
		extensions: []ExtensionInterface{
			new(extensionCounters),
			new(extensionMeta),
			new(extensionDuplicates),
			new(extensionGitHub),
			new(extensionBtc),
			new(extensionReddit),
			new(extensionMovies),
			new(extensionRaw),
		},
	}
	// Logging configuration.
	formatNorm := logging.MustStringFormatter(
		"%{color}[%{time:2006/01/02 15:04:05}] %{level:.4s} ▶%{color:reset} %{message}",
	)
	backendNorm := logging.NewLogBackend(os.Stdout, "", 0)
	backendNormFormatted := logging.NewBackendFormatter(backendNorm, formatNorm)
	logging.SetBackend(backendNormFormatted)

	// Load config.
	if _, err := toml.DecodeFile(bot.configFile, &bot.Config); err != nil {
		bot.log.Fatal("Can't load config: %s", err)
	}

	// Load texts.
	if err := bot.LoadTexts(bot.textsFile, bot.Texts); err != nil {
		bot.log.Fatal("Can't load texts: %s", err)
	}

	return bot
}

// initialize performs initialization of bot's mechanisms.
func (bot *Bot) initialize() {
	bot.log.Info("I am papaBot, version %s", Version)

	// Init database.
	if err := bot.initDb(); err != nil {
		bot.log.Fatal("Can't init database: %s", err)
	}
	bot.ensureOwnerExists()

	// Create log folder.
	if bot.Config.ChatLogging {
		exists, err := utils.DirExists("logs")
		if err != nil {
			bot.log.Fatal("Can't check if logs dir exists: %s", err)
		}
		if !exists {
			if err := os.Mkdir("logs", 0700); err != nil {
				bot.log.Fatal("Can't create logs folder: %s", err)
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
	bot.log.Debug("Next daily tick: %s", bot.nextDailyTick)

	// Load stopwords.
	if err := lexical.LoadStopWords(bot.Config.Language); err != nil {
		bot.log.Warning("Can't load stop words for language %s: %s", bot.Config.Language, err)
		bot.log.Warning("Lexical functions will be handicapped.")
	}
	bot.log.Debug("Loaded %d stopwords for language %s.", len(lexical.StopWords), bot.Config.Language)

	// Init extensions.
	for i := range bot.extensions {
		if err := bot.extensions[i].Init(bot); err != nil {
			bot.log.Fatal("Error loading extensions: %s", err)
		}
	}

	bot.initDone = true
	bot.log.Info("Bot init done.")
}

// connect attempts to connect to the given IRC server.
func (bot *Bot) connect() error {
	var conn net.Conn
	var err error
	// Establish the connection.
	bot.log.Info("Connecting to %s...", bot.Config.Server)
	if bot.Config.TLSConfig == nil {
		conn, err = net.Dial("tcp", bot.Config.Server)
	} else {
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

	bot.log.Debug("Succesfully connected.")
	return nil
}

// receiverLoop attempts to read from the IRC server and keep the connection open.
func (bot *Bot) receiverLoop() {
	for {
		bot.irc.connection.SetDeadline(time.Now().Add(300 * time.Second))
		msg, err := bot.irc.decoder.Decode()
		if err != nil { // Error or timeout.
			bot.log.Warning("Disconnected from server.")
			bot.irc.connection.Close()
			retries := 0
			for {
				time.Sleep(bot.Config.ReconnectDelay * time.Duration(retries))
				bot.log.Info("Reconnecting...")
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

// SendRawMessage sends raw command to the server.
func (bot *Bot) SendRawMessage(command string, params []string, trailing string) {
	if err := bot.irc.encoder.Encode(&irc.Message{
		Command:  command,
		Params:   params,
		Trailing: trailing,
	}); err != nil {
		bot.log.Error("Can't send message %s: %s", command, err)
	}
}

// sendFloodProtected is a flood protected message sender.
func (bot *Bot) sendFloodProtected(mType, channel, message string) {
	messages := strings.Split(message, "\n")
	for i := range messages {
		bot.floodSemaphore <- 1
		bot.SendRawMessage(mType, []string{channel}, messages[i])
	}
}

// SendMessage sends a message to the channel.
func (bot *Bot) SendPrivMessage(channel, message string) {
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
	bot.log.Debug("Sending mass notice: %s", message)
	for _, channel := range bot.Config.Channels {
		bot.sendFloodProtected(irc.NOTICE, channel, message)
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
			bot.log.Warning("Can't load var: %s", err)
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
			bot.log.Error("Error opening log file: %s", err)
			return
		}
		defer f.Close()

		scribe := log.New(f, "", log.Ldate|log.Ltime)
		scribe.Println(message...)
	}()
}

// runExtensionTickers will asynchronously run all extension tickers.
func (bot *Bot) runExtensionTickers() {
	// Catch errors.
	defer func() {
		if Debug {
			return
		} // When in debug mode fail on all errors.
		if r := recover(); r != nil {
			bot.log.Error("FATAL ERROR in tickers: %s", r)
		}
	}()

	// Check if it's time for a daily ticker.
	daily := false
	if time.Since(bot.nextDailyTick) >= 0 {
		daily = true
		bot.nextDailyTick = bot.nextDailyTick.Add(24 * time.Hour)
		bot.log.Debug("Daily tick now. Next at %s.", bot.nextDailyTick)
	}

	// Run the tickers.
	for i := range bot.extensions {
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
		bot.log.Fatal("Error creating connection: ", err)
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
				if bot.eventHandlers[msg.Command] != nil {
					bot.eventHandlers[msg.Command](msg)
				}
			} else {
				break
			}
		}
	}

	bot.log.Info("Exiting...")
}
