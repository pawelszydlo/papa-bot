package extensions

import (
	"fmt"
	"github.com/pawelszydlo/papa-bot"
	"github.com/pawelszydlo/papa-bot/events"
	"regexp"
	"strings"
)

// ExtensionTwitterThread - extension for getting thread links from tweets.
type ExtensionTwitterThread struct {
	twitterRe *regexp.Regexp
	bot       *papaBot.Bot
	lastTweet string
}

// Init inits the extension.
func (ext *ExtensionTwitterThread) Init(bot *papaBot.Bot) error {
	ext.twitterRe = regexp.MustCompile(`^https?:\/\/twitter\.com\/(?:#!\/)?(\w+)\/status(?:es)?\/(\d+).*$`)
	ext.bot = bot

	bot.EventDispatcher.RegisterListener(events.EventURLFound, ext.UrlListener)

	// Register new command.
	bot.RegisterCommand(&papaBot.BotCommand{
		[]string{"tt", "tthread"},
		false, false, false,
		"", "Get a link to a thread version of the last tweet.",
		ext.commandTThread})
	return nil
}

func (ext *ExtensionTwitterThread) extractTweetId(message string) string {
	match := ext.twitterRe.FindStringSubmatch(message)
	if len(match) < 3 {
		return ""
	}
	// Valid Twitter link, store it.
	ext.bot.Log.Infof("Found tweet link: %s", message)
	return match[2]
}

// UrlListener will check for Twitter links and store them.
func (ext *ExtensionTwitterThread) UrlListener(message events.EventMessage) {
	ext.lastTweet = ext.extractTweetId(message.Message)
}

// commandMovie is a command for getting a readable thread from last tweet.
func (ext *ExtensionTwitterThread) commandTThread(bot *papaBot.Bot, sourceEvent *events.EventMessage, params []string) {
	text := strings.Join(params, " ")
	if text != "" {
		ext.lastTweet = ext.extractTweetId(text)
	}
	if ext.lastTweet == "" {
		return
	}
	notice := fmt.Sprintf("%s: https://threadreaderapp.com/thread/%s", sourceEvent.Nick, ext.lastTweet)
	bot.SendMessage(sourceEvent, notice)
	ext.lastTweet = ""
}
