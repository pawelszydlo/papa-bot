package main

import (
	"flag"
	"github.com/pawelszydlo/papa-bot"
)

var (
	configFile = flag.String("c", "config.ini", "Path to TOML configuration file for the bot.")
	textsFile  = flag.String("t", "texts.ini", "Path to TOML configuration file with the bot texts.")
)

func init() {
	flag.Parse()
}

// Entry point
func main() {
	bot := papaBot.New(*configFile, *textsFile)
	bot.Run()
}
