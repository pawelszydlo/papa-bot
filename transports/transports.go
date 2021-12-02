package transports

import (
	"github.com/pawelszydlo/papa-bot/events"
	"github.com/pelletier/go-toml"
	"github.com/sirupsen/logrus"
)

// Transport definition and related types.
// It is up to the transport to connect, join channels and stay on them (handle kicks etc.).

// Transport interface.
type Transport interface {
	// Name should return the transport's name. This can be called before init!
	Name() string
	// Init will always be called after a transport instance is created.
	Init(
		botName string,
		fullConfig *toml.Tree,
		logger *logrus.Logger,
		eventDispatcher *events.EventDispatcher,
	)
	// Will be called once, when the bot starts, and should contain the main loop.
	Run()
	// Check whether a given nick is the transports name for the bot.
	NickIsMe(nick string) bool
	// Gets a list of channels the bot is on.
	GetChannelsOn() []string
	// Gets a list of nicks of users on the channel.
	GetNicks(channel string) []string
	// Send message in reply to sourceEvent.
	SendMessage(sourceEvent *events.EventMessage, message string)
	// Send message in reply to sourceEvent as a direct chat with the user.
	SendPrivateMessage(sourceEvent *events.EventMessage, nick, message string)
	// Send notice in reply to sourceEvent.
	SendNotice(sourceEvent *events.EventMessage, message string)
	// Send notice to all the channels the transport is on.
	SendMassNotice(message string)
}
