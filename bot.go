package papaBot

import (
	"database/sql"
	"encoding/json"
	_ "github.com/mattn/go-sqlite3"
	"github.com/nickvanw/ircx"
	"github.com/sorcix/irc"
	"io/ioutil"
	"log"
	"os"
	"time"
)

type Bot struct {
	irc      *ircx.Bot
	Config   Configuration
	BotOwner string

	Db *sql.DB

	floodSemaphore chan int

	logInfo  *log.Logger
	logWarn  *log.Logger
	logError *log.Logger

	kickedFrom map[string]bool

	processors []func(*Bot, string, string, string)
}

type Configuration struct {
	Server         string
	Name           string
	User           string
	OwnerPassword  string
	Channels       []string
	AntiFloodDelay int
}

// Create new bot
func New(configFile string) *Bot {

	// Init bot struct
	bot := &Bot{
		floodSemaphore: make(chan int, 5),
		logInfo:        log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime),
		logWarn:        log.New(os.Stdout, "WARN: ", log.Ldate|log.Ltime|log.Lshortfile),
		logError:       log.New(os.Stderr, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile),
		kickedFrom:     map[string]bool{},

		Config: Configuration{
			AntiFloodDelay: 5,
		},

		processors: []func(*Bot, string, string, string){processorURLs},
	}

	// Load config
	if err := bot.loadConfig(configFile); err != nil {
		bot.logError.Fatal("Can't load config:", err)
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

	return bot
}

// Load JSON configuration
func (bot *Bot) loadConfig(filename string) error {
	file, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(file, &bot.Config); err != nil {
		return err
	}

	if bot.Config.OwnerPassword == "" { // don't allow empty passwords
		bot.logError.Fatal("You must set OwnerPassword in your config.")
	}

	return nil
}

// Initialize the bot's database
func (bot *Bot) initDb() error {
	db, err := sql.Open("sqlite3", "papabot.db")
	if err != nil {
		return err
	}
	bot.Db = db

	// Create tables if needed
	query := `
		CREATE TABLE IF NOT EXISTS "urls" (
			"id" INTEGER PRIMARY KEY  AUTOINCREMENT  NOT NULL,
			"channel" VARCHAR NOT NULL,
			"nick" VARCHAR NOT NULL,
			"link" VARCHAR NOT NULL,
			"quote" VARCHAR NOT NULL,
			"title" VARCHAR,
			"timestamp" DATETIME DEFAULT (datetime('now','localtime'))
		);`
	if _, err := db.Exec(query); err != nil {
		return err
	}

	return nil
}

// Flush flood semaphore
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

// Flood protected sender
func (bot *Bot) sendFloodProtected(mType, channel, message string) {
	bot.floodSemaphore <- 1
	bot.irc.Sender.Send(&irc.Message{
		Command:  mType,
		Params:   []string{channel},
		Trailing: message,
	})
}

// Send message
func (bot *Bot) SendMessage(channel, message string) {
	go bot.sendFloodProtected(irc.PRIVMSG, channel, message)
}

// Send notice
func (bot *Bot) SendNotice(channel, message string) {
	go bot.sendFloodProtected(irc.NOTICE, channel, message)

}

// Check if the sender is the bot
func (bot *Bot) isMe(name string) bool {
	return name == bot.irc.OriginalName
}

// Cleanup on bot exit
func (bot *Bot) Close() {
	bot.Db.Close()
}

// Main loop
func (bot *Bot) Run() {
	// Connect to server
	if err := bot.irc.Connect(); err != nil {
		bot.logError.Fatal("Error creating connection:", err)
	}

	// Semaphore clearing ticker
	ticker := time.NewTicker(time.Second * time.Duration(bot.Config.AntiFloodDelay))
	go func() {
		for _ = range ticker.C {
			bot.resetFloodSemaphore()
		}
	}()
	defer ticker.Stop()

	bot.irc.HandleLoop()
}
