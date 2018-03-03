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
	Run()
	OnChannels() map[string]bool
	NickIsMe(nick string) bool
	SendMessage(channel, message string)
	SendNotice(channel, message string)
	SendPrivMessage(user, message string)
	SendPrivNotice(user, message string)
	SendMassNotice(message string)
}

// Function type for transport constructors. This function will be registered by the bot.
type NewTransportFunction func(
	transportName string,
	botName string,
	fullConfig *toml.Tree,
	logger *logrus.Logger,
	eventDispatcher *events.EventDispatcher,
) Transport
