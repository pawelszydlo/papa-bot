package extensions

import (
	"github.com/pawelszydlo/papa-bot"
	"strings"
)

// ExtensionRaw - extension for executing raw IRC commands by the bot.
type ExtensionRaw struct {
	Extension
}

func (ext *ExtensionRaw) Init(bot *papaBot.Bot) error {
	// Register new command.
	bot.RegisterCommand(&papaBot.BotCommand{
		[]string{"raw"},
		true, true, false,
		"<command> <params> : [trailing]", "Execute raw IRC command.",
		ext.commandRaw})
	return nil
}

func (ext *ExtensionRaw) commandRaw(bot *papaBot.Bot, nick, user, channel, receiver, transport string, priv bool, params []string) {
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
	bot.Log.Debugf("Executing raw command: %s, params: %v, trailing: %s", command, arguments, trailing)
	// TODO: figure it out!
	//bot.SendRawMessage(command, params, trailing)
}
