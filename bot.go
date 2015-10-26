package main

import (
	"github.com/thoj/go-ircevent"
	_ "github.com/mattn/go-sqlite3"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
	"flag"
	"database/sql"
	"math/rand"
)

var (
	linfo    *log.Logger
	lwarn *log.Logger
	lerror   *log.Logger
	IRC      *irc.Connection
	config	 Configuration
	db		 *sql.DB
)

func InitLogger() {
	linfo = log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime)
	lwarn = log.New(os.Stdout, "WARN: ", log.Ldate|log.Ltime|log.Lshortfile)
	lerror = log.New(os.Stderr, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)
}

// Handle all incoming messages
func HandleMessage(channel, sender, msg string) {
	// silence any errors :)
//	defer func() {
//		if r := recover(); r != nil {
//			lerror.Println("FATAL ERROR in HandleMessage: ", r)
//		}
//	}()

	// is someone talking to the bot?
	true_nick := IRC.GetNick()
	if strings.HasPrefix(msg, true_nick) {
		msg = strings.TrimLeft(msg[len(true_nick):], ",:; ")
		HandleBotCommand(channel, sender, msg)
	}

	// Look for urls
	HandleURLs(channel, sender, msg)
}

// Wait and rejoin the channel
func Rejoin(channel string, delay uint) {
	time.Sleep(time.Duration(delay) * time.Second)
	IRC.Join(channel)
}

// Attach all the handlers
func AttachHandlers() {
	// After server connection
	IRC.AddCallback("001", func (e *irc.Event) {
		for _, channel := range config.Channels {
			IRC.Join(channel)
		}
	})
	// Say hello after joining a channel
	IRC.AddCallback("JOIN", func (e *irc.Event) {
		if e.Nick == IRC.GetNick() {
			linfo.Println("I joined",e.Arguments[0])
			IRC.Privmsg(e.Arguments[0], config.Hellos[rand.Intn(len(config.Hellos))])
		} else {
			linfo.Println(e.Nick, "joined", e.Arguments[0])
		}
	})
	// Bot was kicked
	IRC.AddCallback("KICK", func (e *irc.Event) {
		if e.Nick == IRC.GetNick() {
			linfo.Println("I was kicked from", e.Arguments[0])
			go Rejoin(e.Arguments[0], 3)
		} else {
			linfo.Println(e.Nick, "has kicked", e.Arguments[1], "from", e.Arguments[0])
		}
	})
	// Handling of channel chat
	IRC.AddCallback("PRIVMSG", func (e *irc.Event) {
		go HandleMessage(e.Arguments[0], e.Nick, e.Message())
	})
}

// Entry point
func main() {
	InitLogger()

	// Handle parameters
	configFile := flag.String("c", "config.json", "Path to JSON configuration file for the bot.")
	flag.Parse()

	// Load config
	linfo.Println("Loading bot config from:", *configFile)
	var err error
	config, err = LoadConfig(*configFile)
	if err != nil {
		lerror.Fatal("Error loading config:", err)
	}

	// Open database
	err = InitDatabase()
	if err != nil {
		lerror.Fatal("Error opening the database:", err)
	}
	defer db.Close()

	// Connect to the server
	linfo.Println("Connecting to ", config.Server)
	IRC = irc.IRC(config.Name, config.User)
	IRC.Log = log.New(os.Stdout, "IRCE: ", log.Ldate|log.Ltime)  // Override logger
	if IRC == nil {
		lerror.Fatal("Error creating connection.")
	}
	err = IRC.Connect(fmt.Sprintf("%s:%d", config.Server, config.Port))
	if err != nil {
		lerror.Fatal("Failed connecting to ", config.Server, ": ", err)
	}

	AttachHandlers()

	IRC.Loop()
}