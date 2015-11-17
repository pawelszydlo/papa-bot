// Example usage of papaBot.
package main

import (
	"flag"
	"fmt"
	"github.com/pawelszydlo/papa-bot"
	"time"
)

var (
	configFile = flag.String("c", "config.ini", "Path to TOML configuration file for the bot.")
	textsFile  = flag.String("t", "texts.ini", "Path to TOML configuration file with the bot texts.")
)

func init() {
	flag.Parse()
}

// Create your own extension simply by embedding the Extension struct.
type MyExtension struct {
	papaBot.Extension
	startTime time.Time
}

/*
You can overwrite any of the methods:

// Will be run on bot's init or when extension is registered after bot's init.
func (ext *MyExtension) Init(bot *papaBot.Bot) error { return nil }

// Will be run whenever an URL is found in the message.
func (ext *MyExtension) ProcessURL(bot *papaBot.Bot, info *papaBot.UrlInfo, channel, sender, msg string) {}

// Will be run on every public message the bot receives.
func (ext *MyExtension) ProcessMessage(bot *papaBot.Bot, channel, nick, msg string) {}

// Will be run every 5 minutes. Daily will be set to true once per day.
func (ext *MyExtension) Tick(bot *papaBot.Bot, daily bool) {}

If you need more help please take a look at bot's built in extensions.
*/
func (ext *MyExtension) Init(bot *papaBot.Bot) error {
	ext.startTime = time.Now()
	return nil
}

func (ext *MyExtension) Tick(bot *papaBot.Bot, daily bool) {
	bot.SendMassNotice(
		fmt.Sprintf("I have been running for %.0f minutes now.", time.Since(ext.startTime).Minutes()))
}

// Entry point
func main() {
	bot := papaBot.New(*configFile, *textsFile)
	bot.RegisterExtension(new(MyExtension))
	bot.Run()
}
