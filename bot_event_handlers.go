package papaBot

import (
	"github.com/nickvanw/ircx"
	"github.com/sorcix/irc"
	"math/rand"
	"strings"
	"time"
)

// Attach event handlers to the bot
func (bot *Bot) attachEventHandlers() {
	// Connected to server
	bot.irc.HandleFunc(irc.RPL_WELCOME, bot.HandlerConnect)
	// Ping
	bot.irc.HandleFunc(irc.PING, bot.HanderPing)
	// Join channel
	bot.irc.HandleFunc(irc.JOIN, bot.HandlerJoin)
	// Kick from channel
	bot.irc.HandleFunc(irc.KICK, bot.HandlerKick)
	// Message on channel
	bot.irc.HandleFunc(irc.PRIVMSG, bot.HandlerMsg)
	// Notice
	bot.irc.HandleFunc(irc.NOTICE, bot.HandlerDummy)
	// Error
	bot.irc.HandleFunc(irc.ERROR, bot.HandlerError)
}

func (bot *Bot) HandlerConnect(s ircx.Sender, m *irc.Message) {
	bot.logInfo.Println("I have connected. Joining channels...")
	s.Send(&irc.Message{
		Command: irc.JOIN,
		Params:  bot.Config.Channels,
	})
}

func (bot *Bot) HanderPing(s ircx.Sender, m *irc.Message) {
	s.Send(&irc.Message{
		Command:  irc.PONG,
		Params:   m.Params,
		Trailing: m.Trailing,
	})
}

func (bot *Bot) HandlerError(s ircx.Sender, m *irc.Message) {
	bot.logError.Println("Error from server:", m.Trailing)
}

func (bot *Bot) HandlerDummy(s ircx.Sender, m *irc.Message) {
	bot.logInfo.Printf("MESSAGE: %+v", m)
}

func (bot *Bot) HandlerJoin(s ircx.Sender, m *irc.Message) {
	if bot.isMe(m.Prefix.Name) {
		if kicked, exists := bot.kickedFrom[m.Trailing]; exists && kicked {
			bot.logInfo.Println("I have rejoined", m.Trailing)
			bot.SendMessage(m.Trailing, txtHellosAfterKick[rand.Intn(len(txtHellosAfterKick))])
			delete(bot.kickedFrom, m.Trailing)
		} else {
			bot.logInfo.Println("I have joined", m.Trailing)
			bot.SendMessage(m.Trailing, txtHellos[rand.Intn(len(txtHellosAfterKick))])
		}
	} else {
		bot.logInfo.Println(m.Prefix.Name, "has joined", m.Trailing)
	}
}

func (bot *Bot) HandlerKick(s ircx.Sender, m *irc.Message) {
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
}

func (bot *Bot) HandlerMsg(s ircx.Sender, m *irc.Message) {
	// Silence any errors :)
	defer func() {
		if r := recover(); r != nil {
			bot.logError.Println("FATAL ERROR in HandlerMsg: ", r)
		}
	}()

	msg := m.Trailing
	sender := m.Prefix.Name
	sender_user := m.Prefix.User
	channel := m.Params[0]

	// Is it a private query?
	if bot.isMe(channel) { // private msg to the bot
		go bot.HandleBotCommand(channel, sender, sender_user, msg)
	} else { // message on a channel

		// Is someone talking to the bot?
		true_nick := bot.irc.OriginalName
		if strings.HasPrefix(msg, true_nick) {
			msg = strings.TrimLeft(msg[len(true_nick):], ",:; ")
			go bot.HandleBotCommand(channel, sender, sender_user, msg)
		}

		// Run all the processors
		for _, processor := range bot.processors {
			go processor(bot, channel, sender, msg)
		}
	}

}
