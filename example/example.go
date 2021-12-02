// Example usage of papaBot.
package main

import (
	"flag"
	"fmt"
	"github.com/pawelszydlo/papa-bot"
	"github.com/pawelszydlo/papa-bot/events"
	"github.com/pawelszydlo/papa-bot/extensions"
	"time"
)

var (
	configFile = flag.String("c", "config.ini", "Path to TOML configuration file for the bot.")
	textsFile  = flag.String("t", "texts.ini", "Path to TOML configuration file with the bot texts.")
)

func init() {
	flag.Parse()
}

// Create your own extension.
type MyExtension struct {
	startTime time.Time
	bot       *papaBot.Bot
}

// Will be run on bot's init or when extension is registered after bot's init. This is the only required function
// that your extension must have. For a list of actions your extension can perform on the bot see api.go.
func (ext *MyExtension) Init(bot *papaBot.Bot) error {
	ext.bot = bot
	ext.startTime = time.Now()
	// This is an example event listener registration. You can find a list of events in the "events" package.
	bot.EventDispatcher.RegisterListener(events.EventTick, ext.TickListener)
	// Register new command. See the struct for field descriptions.
	bot.RegisterCommand(&papaBot.BotCommand{
		[]string{"hello"},
		false, false, false,
		"", "Say hello!",
		ext.commandHello})
	return nil
}

// TickListener will be attached to a EventTick event that is happening every 5 minutes.
func (ext *MyExtension) TickListener(message events.EventMessage) {
	ext.bot.SendMassNotice(
		fmt.Sprintf("I have been running for %.0f minutes now.", time.Since(ext.startTime).Minutes()))
}

// commandHello is a command for saying hello.
func (ext *MyExtension) commandHello(bot *papaBot.Bot, sourceEvent *events.EventMessage, params []string) {
	bot.SendMessage(sourceEvent, "Hello!")
}

// Entry point
func main() {
	// This will create bot's structures. Feel free to modify what you need afterwards.
	_, bot := papaBot.New(*configFile, *textsFile)

	// As an example, change the name.
	bot.Config.Name = "David"

	// Add all built-in extensions.
	extensions.RegisterBuiltinExtensions(bot)

	// Add your own custom extension.
	bot.RegisterExtension(new(MyExtension))

	// This will init the bot's mechanisms and run the bot's main loop.
	bot.Run()
}
