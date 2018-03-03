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
	// Init will always be called after a transport instance is created.
	Init(
		transportName string,
		botName string,
		fullConfig *toml.Tree,
		logger *logrus.Logger,
		eventDispatcher *events.EventDispatcher,
	)
	// Will be called once, when the bot starts, and should contain the main loop.
	Run()
	// check whether a given nick is the transport.
	NickIsMe(nick string) bool
	// Send messages.
	SendMessage(channel, message, context string)
	SendNotice(channel, message, context string)
	SendPrivMessage(user, message, context string)
	SendPrivNotice(user, message, context string)
	SendMassNotice(message string)
}
