package transports

// Transport interface.
type Transport interface {
	Run()
	OnChannels() map[string]bool
	NickIsMe(nick string) bool
	SendMessage(channel, message string)
	SendNotice(channel, message string)
	SendPrivMessage(user, message string)
	SendMassNotice(message string)
}

// Message for scribe channel.
type ScribeMessage struct {
	Who, Where, What string
	// Is it a special action or just a chat?
	Special bool
}

// Message for commands channel - that is any message directed at the bot.
type CommandMessage struct {
	Channel, Nick string
	// Full username of the sender.
	UserName string
	// Message contents.
	Message string
	// Should the bot talk back when something is wrong?
	TalkBack bool
}
