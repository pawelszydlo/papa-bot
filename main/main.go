package main

import (
	"github.com/pawelszydlo/papa-bot"
	"flag"
)

var (
	configFile = flag.String("c", "config.json", "Path to JSON configuration file for the bot.")
)

func init() {
	flag.Parse()
}


// Entry point
func main() {
	bot := papaBot.New(*configFile)
	bot.Run()
	defer bot.Close()
}