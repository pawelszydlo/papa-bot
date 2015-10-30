package papaBot

import (
	"fmt"
	"github.com/nickvanw/ircx"
	"github.com/sorcix/irc"
	"math/rand"
	"strings"
	"time"
)

// attachEventHandlers attaches event handlers to the bot.
func (bot *Bot) attachEventHandlers() {
	// Connected to server
	bot.irc.HandleFunc(irc.RPL_WELCOME, bot.handlerConnect)
	// Ping
	bot.irc.HandleFunc(irc.PING, bot.handlerPing)
	// Join channel
	bot.irc.HandleFunc(irc.JOIN, bot.handlerJoin)
	// Part channel
	bot.irc.HandleFunc(irc.PART, bot.handlerPart)
	// Set mode
	bot.irc.HandleFunc(irc.MODE, bot.handlerMode)
	// Set topic
	bot.irc.HandleFunc(irc.TOPIC, bot.handlerTopic)
	// Kick from channel
	bot.irc.HandleFunc(irc.KICK, bot.handlerKick)
	// Message on channel
	bot.irc.HandleFunc(irc.PRIVMSG, bot.handlerMsg)
	// Notice
	bot.irc.HandleFunc(irc.NOTICE, bot.handlerDummy)
	// Error
	bot.irc.HandleFunc(irc.ERROR, bot.handlerError)
}

func (bot *Bot) handlerConnect(s ircx.Sender, m *irc.Message) {
	bot.logInfo.Println("I have connected. Joining channels...")
	s.Send(&irc.Message{
		Command: irc.JOIN,
		Params:  bot.Config.Channels,
	})
}

func (bot *Bot) handlerPing(s ircx.Sender, m *irc.Message) {
	s.Send(&irc.Message{
		Command:  irc.PONG,
		Params:   m.Params,
		Trailing: m.Trailing,
	})
}

func (bot *Bot) handlerPart(s ircx.Sender, m *irc.Message) {
	bot.logInfo.Println(m.Prefix.Name, "has left", m.Params[0], ":", m.Trailing)
	bot.Scribe(m.Params[0], m.Prefix.Name, "has left", m.Params[0], ":", m.Trailing)
}

func (bot *Bot) handlerError(s ircx.Sender, m *irc.Message) {
	bot.logError.Println("Error from server:", m.Trailing)
}

func (bot *Bot) handlerDummy(s ircx.Sender, m *irc.Message) {
	bot.logInfo.Printf("MESSAGE: %+v", m)
}

func (bot *Bot) handlerJoin(s ircx.Sender, m *irc.Message) {
	if bot.isMe(m.Prefix.Name) {
		if bot.kickedFrom[m.Trailing] {
			bot.logInfo.Println("I have rejoined", m.Trailing)
			bot.SendMessage(m.Trailing, bot.Texts.HellosAfterKick[rand.Intn(len(bot.Texts.HellosAfterKick))])
			delete(bot.kickedFrom, m.Trailing)
		} else {
			bot.logInfo.Println("I have joined", m.Trailing)
			bot.SendMessage(m.Trailing, bot.Texts.Hellos[rand.Intn(len(bot.Texts.Hellos))])
		}
	} else {
		bot.logInfo.Println(m.Prefix.Name, "has joined", m.Trailing)
	}
	bot.Scribe(m.Trailing, m.Prefix.Name, "has joined", m.Trailing)
}

func (bot *Bot) handlerMode(s ircx.Sender, m *irc.Message) {
	bot.logInfo.Println(m.Prefix.Name, "has set mode", m.Params[1:], "on", m.Params[0])
	bot.Scribe(m.Params[0], m.Prefix.Name, "has set mode", m.Params[1:], "on", m.Params[0])
}

func (bot *Bot) handlerTopic(s ircx.Sender, m *irc.Message) {
	bot.logInfo.Println(m.Prefix.Name, "has set topic on", m.Params[0], "to:", m.Trailing)
	bot.Scribe(m.Params[0], m.Prefix.Name, "has set topic on", m.Params[0], "to:", m.Trailing)
}

func (bot *Bot) handlerKick(s ircx.Sender, m *irc.Message) {
	if bot.isMe(m.Params[1]) {
		bot.logInfo.Println("I was kicked from", m.Params[0], "by", m.Prefix.Name, "for:", m.Trailing)
		bot.kickedFrom[m.Params[0]] = true
		// Rejoin
		timer := time.NewTimer(3 * time.Second)
		go func() {
			<-timer.C
			s.Send(&irc.Message{
				Command: irc.JOIN,
				Params:  []string{m.Params[0]},
			})
		}()
	} else {
		bot.logInfo.Println(m.Prefix.Name, "has kicked", m.Params[1], "from", m.Params[0], "for:", m.Trailing)
	}
	bot.Scribe(m.Params[0], m.Prefix.Name, "has kicked", m.Params[1], "from", m.Params[0], "for:", m.Trailing)
}

func (bot *Bot) handlerMsg(s ircx.Sender, m *irc.Message) {
	// Silence any errors :)
	defer func() {
		if r := recover(); r != nil {
			bot.logError.Println("FATAL ERROR in handlerMsg: ", r)
		}
	}()

	msg := m.Trailing
	if msg == "" {
		return
	}
	nick := m.Prefix.Name
	user := m.Prefix.User + "@" + m.Prefix.Host
	channel := m.Params[0]

	if bot.isMe(nick) { // It's the bot talking
		return
	}

	// Special CTCP
	if strings.HasPrefix(msg, "\x01") && strings.HasSuffix(msg, "\x01") {
		msg := msg[1 : len(msg)-1]

		if msg == "VERSION" {
			bot.logDebug.Println("Replying to VERSION query from", nick)
			bot.SendNotice(nick, fmt.Sprintf("\x01VERSION papaBot:%s:Go bot running on insomnia.\x01", Version))
			return
		}

		if msg == "FINGER" {
			bot.logDebug.Println("Replying to FINGER query from", nick)
			bot.SendNotice(nick, "\x01FINGER yourself.\x01")
			return
		}

		bot.logDebug.Println(nick, "sent a", msg, "CTCP request. Ignoring.")
		return
	}

	// Is it a private query?
	if bot.isMe(channel) { // private msg to the bot
		go bot.handleBotCommand(channel, nick, user, msg)
	} else { // Message on a channel
		bot.Scribe(channel, fmt.Sprintf("<%s> %s", nick, msg))

		// Is someone talking to the bot?
		true_nick := bot.irc.OriginalName
		if strings.HasPrefix(msg, true_nick) {
			msg = strings.TrimLeft(msg[len(true_nick):], ",:; ")
			if msg != "" {
				go bot.handleBotCommand(channel, nick, user, msg)
				return
			}
		}
		// Maybe a dot command?
		if strings.HasPrefix(msg, ".") {
			msg = strings.TrimPrefix(msg, ".")
			if msg != "" {
				go bot.handleBotCommand(channel, nick, user, msg)
				return
			}
		}
		// Run all the processors
		processorURLs(bot, channel, nick, msg)
	}

}
