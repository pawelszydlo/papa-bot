// Extensions for papaBot.
package extensions

import "github.com/pawelszydlo/papa-bot"

// Extensions should embed this struct.
type Extension struct{}

// Will be run on bot's init or when extension is registered after bot's init.
func (ext *Extension) Init(bot *papaBot.Bot) error { return nil }

// Everything else should be handled through events. See bot.EventDispatcher.

// RegisterBuiltinExtensions will do exactly what you think it will do.
func RegisterBuiltinExtensions(bot *papaBot.Bot) {
	bot.RegisterExtension(new(ExtensionCounters))
	bot.RegisterExtension(new(ExtensionDuplicates))
	bot.RegisterExtension(new(ExtensionGitHub))
	bot.RegisterExtension(new(ExtensionBtc))
	bot.RegisterExtension(new(ExtensionReddit))
	bot.RegisterExtension(new(ExtensionMovies))
	bot.RegisterExtension(new(ExtensionWiki))
	bot.RegisterExtension(new(ExtensionWolfram))
	bot.RegisterExtension(new(ExtensionTalk))
}
