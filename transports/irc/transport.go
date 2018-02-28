package ircTransport

// IRC transport for papaBot.

import (
	"strings"
	"time"

	"crypto/tls"
	"net"

	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/pawelszydlo/papa-bot/transports"
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
	scribeChannel chan transports.ScribeMessage
	// Commands channel
	commandChannel chan transports.CommandMessage

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
func (transport *IRCTransport) SendMessage(channel, message string) {
	transport.log.Debugf("Sending message to %s: %s", channel, message)
	transport.sendFloodProtected(irc.PRIVMSG, channel, message)
}

// SendNotice sends a notice to the channel.
func (transport *IRCTransport) SendNotice(channel, message string) {
	transport.log.Debugf("Sending notice to %s: %s", channel, message)
	transport.sendFloodProtected(irc.NOTICE, channel, message)
}

// SendMessage sends a message to the channel.
func (transport *IRCTransport) SendPrivMessage(user, message string) {
	// In IRC, priv messages are the same as channel messages.
	transport.SendMessage(user, message)
}

// SendMassNotice sends a notice to all the channels transport is on.
func (transport *IRCTransport) SendMassNotice(message string) {
	transport.log.Debugf("Sending mass notice: %s", message)
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

// OnChannels will return a map of all channels the transport is on.
func (transport *IRCTransport) OnChannels() map[string]bool {
	return transport.onChannel
}

// NickIsMe checks if the sender is the transport.
func (transport *IRCTransport) NickIsMe(nick string) bool {
	return nick == transport.name
}

// scribe forwards the message to the bot for logging.
func (transport *IRCTransport) scribe(special bool, channel string, who string, message ...interface{}) {
	scribeMessage := transports.ScribeMessage{
		who,
		channel,
		fmt.Sprint(message...),
		special,
	}
	transport.scribeChannel <- scribeMessage
}

func (transport *IRCTransport) handleCommand(channel, nick, user, msg string, talkBack bool) {
	command := transports.CommandMessage{
		channel,
		nick,
		user,
		msg,
		talkBack,
	}
	transport.commandChannel <- command
}
