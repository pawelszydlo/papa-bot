package main
import "strings"

// Handle command directed at the bot
func HandleBotCommand(channel, sender, sender_user, command string) {
	if command == "" {
		return
	}
	// Was this command sent on a private query?
	private := false
	if isMe(channel) {
		private = true
	}
	// Was the command sent by the owner?
	userstr := sender + "!" + sender_user
	owner := false
	if userstr == botOwner {
		owner = true
	}

	linfo.Println("Command from:", sender, "on:", channel, "command:", command, "private:", private, "owner:", owner)

	params := strings.Split(command, " ")
	command = params[0]
	params = params[1:]

	if command == "auth" {
		if !private {
			SendMessage(channel, sender + ", " + txtNeedsPriv)
			return
		}
		if len(params) == 1 && params[0] == config.OwnerPassword {
			SendMessage(sender, txtPasswordOk)
			botOwner = userstr
			linfo.Println("Owner set to:", botOwner)
		}
	}
}