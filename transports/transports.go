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
	// Report the channels the transport is on.
	OnChannels() map[string]bool
	// check whether a given nick is the transport.
	NickIsMe(nick string) bool
	// Send messages.
	SendMessage(channel, message string)
	SendNotice(channel, message string)
	SendPrivMessage(user, message string)
	SendPrivNotice(user, message string)
	SendMassNotice(message string)
}
