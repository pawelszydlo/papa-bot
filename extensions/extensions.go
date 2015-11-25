// Extensions for papaBot.
package extensions

import "github.com/pawelszydlo/papa-bot"

// Extensions should embed this struct and override any methods necessary.
type Extension struct{}

// Will be run on bot's init or when extension is registered after bot's init.
func (ext *Extension) Init(bot *papaBot.Bot) error { return nil }

// Will be run whenever an URL is found in the message.
func (ext *Extension) ProcessURL(bot *papaBot.Bot, channel, sender, msg string, urlinfo *papaBot.UrlInfo) {
}

// Will be run on every public message the bot receives.
func (ext *Extension) ProcessMessage(bot *papaBot.Bot, channel, sender, msg string) {}

// Will be run every 5 minutes. Daily will be set to true only once per day.
func (ext *Extension) Tick(bot *papaBot.Bot, daily bool) {}

// RegisterBuiltinExtensions will do exactly what you think it will do.
func RegisterBuiltinExtensions(bot *papaBot.Bot) {
	bot.RegisterExtension(new(ExtensionCounters))
	bot.RegisterExtension(new(ExtensionMeta))
	bot.RegisterExtension(new(ExtensionDuplicates))
	bot.RegisterExtension(new(ExtensionGitHub))
	bot.RegisterExtension(new(ExtensionBtc))
	bot.RegisterExtension(new(ExtensionReddit))
	bot.RegisterExtension(new(ExtensionMovies))
	bot.RegisterExtension(new(ExtensionRaw))
	bot.RegisterExtension(new(ExtensionWiki))
}
