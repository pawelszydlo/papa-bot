// Extensions for papaBot.
package extensions

import "github.com/pawelszydlo/papa-bot"

// All extensions need to fit papaBot.extension interface.

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
	bot.RegisterExtension(new(ExtensionAqicn))
	bot.RegisterExtension(new(ExtensionReminders))
	bot.RegisterExtension(new(ExtensionYoutube))
	bot.RegisterExtension(new(ExtensionTwitterThread))
	bot.RegisterExtension(new(ExtensionLastSpoken))
}
