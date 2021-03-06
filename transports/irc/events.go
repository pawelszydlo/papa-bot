package ircTransport

// Handlers for IRC events.

import (
	"github.com/pawelszydlo/papa-bot/events"
	"github.com/sorcix/irc"
	"strings"
	"time"
)

// assignEventHandlers assigns appropriate event handlers.
func (transport *IRCTransport) assignEventHandlers() {
	// Connected to server
	transport.registerIrcEventHandler(irc.RPL_WELCOME, handlerConnect)
	// Ping
	transport.registerIrcEventHandler(irc.PING, handlerPing)
	// Nickname taken
	transport.registerIrcEventHandler(irc.ERR_NICKCOLLISION, handlerNickTaken)
	transport.registerIrcEventHandler(irc.ERR_NICKNAMEINUSE, handlerNickTaken)
	// Invalid nickname
	transport.registerIrcEventHandler(irc.ERR_NONICKNAMEGIVEN, handlerBadNick)
	transport.registerIrcEventHandler(irc.ERR_ERRONEUSNICKNAME, handlerBadNick)
	// Various events that prevent the transport from joining a channel
	transport.registerIrcEventHandler(irc.ERR_CHANNELISFULL, handlerCantJoin)
	transport.registerIrcEventHandler(irc.ERR_BANNEDFROMCHAN, handlerCantJoin)
	transport.registerIrcEventHandler(irc.ERR_INVITEONLYCHAN, handlerCantJoin)
	// Join channel
	transport.registerIrcEventHandler(irc.JOIN, handlerJoin)
	// Part channel
	transport.registerIrcEventHandler(irc.PART, handlerPart)
	// Set mode
	transport.registerIrcEventHandler(irc.MODE, handlerMode)
	// Set topic
	transport.registerIrcEventHandler(irc.TOPIC, handlerTopic)
	// Kick from channel
	transport.registerIrcEventHandler(irc.KICK, handlerKick)
	// Message on channel
	transport.registerIrcEventHandler(irc.PRIVMSG, handlerMsg)
	// Notice
	transport.registerIrcEventHandler(irc.NOTICE, handlerNotice)
	// Error
	transport.registerIrcEventHandler(irc.ERROR, handlerError)
}

func handlerConnect(transport *IRCTransport, m *irc.Message) {
	transport.log.Infof("I have connected. Joining channels...")
	transport.SendRawMessage(irc.JOIN, transport.channels, "")
	transport.sendEvent(events.EventConnected, true, "", transport.name, transport.user, "")
}

func handlerPing(transport *IRCTransport, m *irc.Message) {
	transport.SendRawMessage(irc.PONG, m.Params, m.Trailing)
}

func handlerNickTaken(transport *IRCTransport, m *irc.Message) {
	transport.name = transport.name + "_"
	transport.log.Warningf(
		"Server at %s said that my nick is already taken. Changing nick to %s", m.Prefix.Name, transport.name)
}

func handlerCantJoin(transport *IRCTransport, m *irc.Message) {
	transport.log.Warningf("Server at %s said that I can't join %s: %s", m.Prefix.Name, m.Params[1], m.Trailing)
	// Rejoin
	timer := time.NewTimer(transport.rejoinDelay)
	go func() {
		<-timer.C
		transport.log.Debugf("Trying to join %s...", m.Params[1])
		transport.SendRawMessage(irc.JOIN, []string{m.Params[1]}, "")
	}()
}

func handlerBadNick(transport *IRCTransport, m *irc.Message) {
	transport.log.Fatalf("Server at %s said that my nick is invalid.", m.Prefix.Name)
}

func handlerPart(transport *IRCTransport, m *irc.Message) {
	if transport.NickIsMe(m.Prefix.Name) {
		delete(transport.onChannel, m.Params[0])
	}
	transport.log.Infof("%s has left %s: %s", m.Prefix.Name, m.Params[0], m.Trailing)
	transport.sendEvent(
		events.EventPartChannel, transport.NickIsMe(m.Prefix.Name),
		m.Params[0], m.Prefix.Name, m.Prefix.User, m.Trailing)
}

func handlerError(transport *IRCTransport, m *irc.Message) {
	transport.log.Errorf("Error from server:", m.Trailing)
}

func handlerNotice(transport *IRCTransport, m *irc.Message) {
	transport.log.Infof("MESSAGE: %+v", m)
}

func handlerJoin(transport *IRCTransport, m *irc.Message) {
	if transport.NickIsMe(m.Prefix.Name) {
		if transport.kickedFrom[m.Trailing] {
			transport.log.Infof("I have rejoined %s", m.Trailing)
			transport.sendEvent(events.EventReJoinedChannel, true, m.Trailing, transport.name, transport.user, "")
			delete(transport.kickedFrom, m.Trailing)
		} else {
			transport.log.Infof("I have joined %s", m.Trailing)
			transport.sendEvent(events.EventJoinedChannel, true, m.Trailing, transport.name, transport.user, "")
		}
		transport.onChannel[m.Trailing] = true
	} else {
		transport.log.Infof("%s has joined %s", m.Prefix.Name, m.Trailing)
		transport.sendEvent(events.EventJoinedChannel, false, m.Trailing, m.Prefix.Name, m.Prefix.User, "")
	}
}

func handlerMode(transport *IRCTransport, m *irc.Message) {
	transport.log.Infof("%s has set mode %s on %s", m.Prefix.Name, m.Params[1:], m.Params[0])
	transport.sendEvent(events.EventChannelOps, false, m.Params[0], m.Prefix.Name, m.Prefix.User, "set mode", m.Params[1:], "on", m.Params[0])
}

func handlerTopic(transport *IRCTransport, m *irc.Message) {
	transport.log.Infof("%s has set topic on %s to: %s", m.Prefix.Name, m.Params[0], m.Trailing)
	transport.sendEvent(events.EventChannelOps, false, m.Params[0], m.Prefix.Name, m.Prefix.User, "set topic on", m.Params[0], "to:", m.Trailing)
}

func handlerKick(transport *IRCTransport, m *irc.Message) {
	if transport.NickIsMe(m.Params[1]) {
		transport.log.Infof("I was kicked from %s by %s for: %s", m.Params[0], m.Prefix.Name, m.Trailing)
		transport.sendEvent(events.EventKickedFromChannel, true, m.Params[0], m.Prefix.Name, m.Prefix.User, m.Trailing)
		transport.kickedFrom[m.Params[0]] = true
		delete(transport.onChannel, m.Params[0])
		// Rejoin
		timer := time.NewTimer(transport.rejoinDelay)
		go func() {
			<-timer.C
			transport.SendRawMessage(irc.JOIN, []string{m.Params[0]}, "")
		}()
	} else {
		transport.log.Infof("%s was kicked from %s by %s for: %s", m.Params[1], m.Prefix.Name, m.Params[0], m.Trailing)
		transport.sendEvent(events.EventKickedFromChannel, false, m.Prefix.Name, m.Params[0], "", "%s kicked for: %s", m.Params[1], m.Trailing)
	}
}

func handlerMsg(transport *IRCTransport, m *irc.Message) {
	msg := m.Trailing
	if msg == "" {
		return
	}
	nick := m.Prefix.Name
	user := m.Prefix.User + "@" + m.Prefix.Host
	channel := m.Params[0]

	if transport.NickIsMe(nick) { // It's the transport talking
		return
	}

	// Special CTCP
	if strings.HasPrefix(msg, "\x01") && strings.HasSuffix(msg, "\x01") {
		msg := msg[1 : len(msg)-1]

		if msg == "VERSION" {
			transport.log.Debugf("Replying to VERSION query from %s...", nick)
			transport.sendIRCNotice(nick, "\x01VERSION papaBot running on insomnia.\x01")
			return
		}

		if msg == "FINGER" {
			transport.log.Debugf("Replying to FINGER query from %s...", nick)
			transport.sendIRCNotice(nick, "\x01FINGER yourself.\x01")
			return
		}

		transport.log.Debugf("%s sent a %s CTCP request. Ignoring.", nick, msg)
		return
	}

	direct := false
	if strings.HasPrefix(msg, transport.name) { // Is someone talking to the bot?
		msg = strings.TrimLeft(msg[len(transport.name):], ",:; ")
		if msg != "" {
			direct = true
		}
	} else if strings.HasPrefix(msg, ".") { // Maybe a dot command?
		msg = strings.TrimPrefix(msg, ".")
		if msg != "" {
			direct = true
		}
	}

	eventCode := events.EventChatMessage
	if !strings.HasPrefix(channel, "#") { // no # prefix means private message.
		eventCode = events.EventPrivateMessage
	}

	// Message on a channel.
	transport.sendEvent(eventCode, direct, channel, nick, user, msg)
}
