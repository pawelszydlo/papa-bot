package main

// Handle command directed at the bot
func HandleBotCommand(channel, sender, command string) {
	linfo.Println("Got command from " + sender + " on " + channel + ": " + command)
}