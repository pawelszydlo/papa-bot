package ircTransport

// IRC transport for papaBot.

import (
	"strings"
	"time"

	"crypto/tls"
	"net"

	"fmt"
	"github.com/pawelszydlo/papa-bot/events"
	"github.com/pawelszydlo/papa-bot/utils"
	"github.com/pelletier/go-toml"
	"github.com/sirupsen/logrus"
	"github.com/sorcix/irc"
)

const MsgLengthLimit = 440 // IRC message length limit.

// Interface for IRC event handler function.
type ircEvenHandlerFunc func(transport *IRCTransport, m *irc.Message)

type IRCTransport struct {
	// Settings.

	// Connection parameters
	server   string
	name     string
	user     string
	password string
	// channels to join.
	channels []string
	// Delay of next messages after flood semaphore hits.
	antiFloodDelay int
	// Delay between rejoin attempts.
	rejoinDelay time.Duration

	// Provided by the bot.

	// Logger.
	log *logrus.Logger
	// Scribe channel
	eventDispatcher *events.EventDispatcher

	// Operational.

	// IRC messages stream.
	messages chan *irc.Message
	// Network connection.
	connection net.Conn
	// IO.
	decoder *irc.Decoder
	encoder *irc.Encoder
	// TLS config.
	tlsConfig *tls.Config
	// Anti flood buffered semaphore
	floodSemaphore chan int
	// Channels bot was kicked from.
	kickedFrom map[string]bool
	// Channels the bot is on.
	onChannel map[string]bool
	// Registered event handlers.
	ircEventHandlers map[string][]ircEvenHandlerFunc
}

// Init initializes a transport instance.
func (transport *IRCTransport) Init(botName string, fullConfig *toml.Tree, logger *logrus.Logger,
	eventDispatcher *events.EventDispatcher,
) {

	// Init the transport struct.
	transport.messages = make(chan *irc.Message)
	transport.antiFloodDelay = 5
	transport.rejoinDelay = 15 * time.Second
	transport.name = botName
	transport.user = fullConfig.GetDefault("irc.user", "papaBot").(string)
	transport.password = fullConfig.GetDefault("irc.password", "").(string)
	transport.server = fullConfig.GetDefault("irc.server", "localhost:6667").(string)
	transport.channels = utils.ToStringSlice(fullConfig.GetDefault("irc.channels", []string{"#papabot"}).([]interface{}))
	// State.
	transport.floodSemaphore = make(chan int, 5)
	transport.kickedFrom = map[string]bool{}
	transport.onChannel = map[string]bool{}
	transport.ircEventHandlers = make(map[string][]ircEvenHandlerFunc)
	// Utility objects.
	transport.log = logger
	transport.eventDispatcher = eventDispatcher

	// Prepare TLS config if needed.
	if fullConfig.GetDefault("irc.use_tls", false).(bool) {
		transport.tlsConfig = &tls.Config{}
		if fullConfig.GetDefault("irc.tls_skip_verify", false).(bool) {
			transport.tlsConfig.InsecureSkipVerify = true
		}
	}

	// Attach event handlers.
	transport.assignEventHandlers()
}

// Name of the transport.
func (transport *IRCTransport) Name() string {
	return "irc"
}

// registerIrcEventHandler will register a new handler for the given IRC event.
func (transport *IRCTransport) registerIrcEventHandler(event string, handler ircEvenHandlerFunc) {
	transport.ircEventHandlers[event] = append(transport.ircEventHandlers[event], handler)
}

// sendRawMessage sends raw command to the server.
func (transport *IRCTransport) SendRawMessage(command string, params []string, trailing string) {
	if err := transport.encoder.Encode(&irc.Message{
		Command:  command,
		Params:   params,
		Trailing: trailing,
	}); err != nil {
		transport.log.Errorf("Can't send message %s: %s", command, err)
	}
}

// SendMessage sends a message to the channel.
func (transport *IRCTransport) SendMessage(sourceEvent *events.EventMessage, message string) {
	transport.sendIRCMessage(sourceEvent.Channel, message)
}

// SendPrivateMessage sends a message to the nick.
func (transport *IRCTransport) SendPrivateMessage(sourceEvent *events.EventMessage, nick, message string) {
	transport.sendIRCMessage(nick, message)
}

// SendNotice sends a notice to the channel.
func (transport *IRCTransport) SendNotice(sourceEvent *events.EventMessage, message string) {
	transport.sendIRCNotice(sourceEvent.Channel, message)
}

// sendIRCMessage sends a message to the channel.
func (transport *IRCTransport) sendIRCMessage(channel, message string) {
	transport.sendFloodProtected(irc.PRIVMSG, channel, message)
}

// sendIRCNotice sends a notice to the channel.
func (transport *IRCTransport) sendIRCNotice(channel, message string) {
	transport.sendFloodProtected(irc.NOTICE, channel, message)
}

// SendMassNotice sends a notice to all the channels transport is on.
func (transport *IRCTransport) SendMassNotice(message string) {
	for _, channel := range transport.getChannelsOn() {
		transport.sendFloodProtected(irc.NOTICE, channel, message)
	}
}

// sendFloodProtected is a flood protected message sender.
func (transport *IRCTransport) sendFloodProtected(mType, channel, message string) {
	messages := strings.Split(message, "\n")
	for i := range messages {
		// IRC message size limit.
		if len(messages[i]) > MsgLengthLimit {
			for n := 0; n < len(messages[i]); n += MsgLengthLimit {
				upperLimit := n + MsgLengthLimit
				if upperLimit > len(messages[i]) {
					upperLimit = len(messages[i])
				}
				transport.floodSemaphore <- 1
				transport.SendRawMessage(mType, []string{channel}, messages[i][n:upperLimit])
			}
			return
		}
		transport.floodSemaphore <- 1
		transport.SendRawMessage(mType, []string{channel}, messages[i])
	}
}

// getChannelsOn will return a list of channels the transport is currently on.
func (transport *IRCTransport) getChannelsOn() []string {
	channelsOn := []string{}
	for channel, on := range transport.onChannel {
		if on {
			channelsOn = append(channelsOn, channel)
		}
	}
	return channelsOn
}

// isOnChannel will check if transport is on the given channel.
func (transport *IRCTransport) isOnChannel(channel string) bool {
	return transport.onChannel[channel]
}

// NickIsMe checks if the sender is the transport.
func (transport *IRCTransport) NickIsMe(nick string) bool {
	return nick == transport.name
}

// sendEvent triggers an event for the bot.
func (transport *IRCTransport) sendEvent(eventCode events.EventCode, direct bool, channel, nick, userId string, message ...interface{}) {
	eventMessage := events.EventMessage{
		transport.Name(),
		events.FormatIRC,
		eventCode,
		nick,
		userId,
		channel,
		fmt.Sprint(message...),
		"",
		direct,
	}
	transport.eventDispatcher.Trigger(eventMessage)
}
