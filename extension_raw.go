package papaBot

import (
	"github.com/sorcix/irc"
	"strings"
)

// ExtensionRaw - extension for executing raw IRC commands by the bot.
type ExtensionRaw struct {
	Extension
}

func (ext *ExtensionRaw) Init(bot *Bot) error {
	// Register new command.
	bot.commands["raw"] = &BotCommand{
		true, true, false,
		"raw [command] [params] : [trailing]", "Execute raw IRC command.",
		ext.commandRaw}
	return nil
}

func (ext *ExtensionRaw) commandRaw(bot *Bot, nick, user, channel, receiver string, priv bool, params []string) {
	if len(params) < 2 {
		return
	}
	command := params[0]
	arguments := strings.Split(strings.Join(params[1:], " "), ":")
	trailing := ""
	if len(arguments) > 1 {
		trailing = strings.Trim(arguments[1], " ")
	}
	arguments = strings.Split(strings.Trim(arguments[0], " "), " ")
	bot.log.Debug("Executing raw command: %s, params: %v, trailing: %s", command, arguments, trailing)
	bot.irc.Sender.Send(&irc.Message{
		Command:  command,
		Params:   params,
		Trailing: trailing,
	})
}
