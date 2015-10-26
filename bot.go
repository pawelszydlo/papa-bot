package main

import (
	"github.com/nickvanw/ircx"
	"flag"
	"github.com/sorcix/irc"
)

var (
	bot      *ircx.Bot
	config	 Configuration
	configFile = flag.String("c", "config.json", "Path to JSON configuration file for the bot.")
	botOwner string
	err	error
)

func init() {
	flag.Parse()
}

// Send message
func SendMessage(channel, message string) {
	bot.Sender.Send(&irc.Message{
		Command:  irc.PRIVMSG,
		Params:   []string{channel},
		Trailing: message,
	})
}

// Send notice
func SendNotice(channel, message string) {
	bot.Sender.Send(&irc.Message{
		Command:  irc.NOTICE,
		Params:   []string{channel},
		Trailing: message,
	})
}

// Entry point
func main() {
	// Load config
	linfo.Println("Loading bot config from:", *configFile)
	var err error
	config, err = LoadConfig(*configFile)
	if err != nil {
		lerror.Fatal("Error loading config:", err)
	}

	// Connect to the server
	linfo.Println("Connecting to", config.Server)
	bot = ircx.Classic(config.Server, config.Name)
	bot.Config.User = config.User
	if err := bot.Connect(); err != nil {
		lerror.Fatal("Error creating connection:", err)
	}

	AttachHandlers()

	bot.HandleLoop()
}