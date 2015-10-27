// Commands for the bot go here.
package papaBot

import "strings"

// Handle command directed at the bot
func (bot *Bot) HandleBotCommand(channel, sender, sender_user, command string) {
	if command == "" {
		return
	}
	// Was this command sent on a private query?
	private := false
	if bot.isMe(channel) {
		private = true
	}
	// Was the command sent by the owner?
	userstr := sender + "!" + sender_user
	owner := false
	if userstr == bot.BotOwner {
		owner = true
	}

	bot.logInfo.Println("Command from:", sender, "on:", channel, "command:", command, "private:", private, "owner:", owner)

	params := strings.Split(command, " ")
	command = params[0]
	params = params[1:]

	if command == "auth" {
		if !private {
			bot.SendMessage(channel, sender+", "+txtNeedsPriv)
			return
		}
		if len(params) == 1 && params[0] == bot.Config.OwnerPassword {
			bot.SendMessage(sender, txtPasswordOk)
			bot.BotOwner = userstr
			bot.logInfo.Println("Owner set to:", bot.BotOwner)
		}
	}
}
