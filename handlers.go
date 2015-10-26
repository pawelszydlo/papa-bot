package main

import (
	"github.com/nickvanw/ircx"
	"github.com/sorcix/irc"
	"strings"
	"math/rand"
	"time"
)

var (
	kickedFrom = map[string]bool{}
)

// Attach event handlers to the bot
func AttachHandlers() {
	// Connected to server
	bot.HandleFunc(irc.RPL_WELCOME, HandlerConnect)
	// Ping
	bot.HandleFunc(irc.PING, HanderPing)
	// Join channel
	bot.HandleFunc(irc.JOIN, HandlerJoin)
	// Kick from channel
	bot.HandleFunc(irc.KICK, HandlerKick)
	// Message on channel
	bot.HandleFunc(irc.PRIVMSG, HandlerMsg)
}

func HandlerConnect(s ircx.Sender, m *irc.Message) {
	linfo.Println("I have connected. Joining channels...")
	s.Send(&irc.Message{
		Command: irc.JOIN,
		Params:  config.Channels,
	})
}

func HanderPing(s ircx.Sender, m *irc.Message) {
	s.Send(&irc.Message{
		Command:  irc.PONG,
		Params:   m.Params,
		Trailing: m.Trailing,
	})
}

func HandlerJoin(s ircx.Sender, m *irc.Message) {
	if isMe(m.Prefix.Name) {
		if kicked, exists := kickedFrom[m.Trailing]; exists && kicked {
			linfo.Println("I have rejoined", m.Trailing)
			SendMessage(m.Trailing, txtHellosAfterKick[rand.Intn(len(txtHellosAfterKick))])
			kickedFrom[m.Trailing] = false
		} else {
			linfo.Println("I have joined", m.Trailing)
			SendMessage(m.Trailing, txtHellos[rand.Intn(len(txtHellosAfterKick))])
		}
	} else {
		linfo.Println(m.Prefix.Name, "has joined", m.Trailing)
	}
}

func HandlerKick(s ircx.Sender, m *irc.Message) {
	if isMe(m.Params[1]) {
		linfo.Println("I was kicked from", m.Params[0], "by", m.Prefix.Name, "for:", m.Trailing)
		kickedFrom[m.Params[0]] = true
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
		linfo.Println(m.Prefix.Name, "has kicked", m.Params[1], "from", m.Params[0], "for:", m.Trailing)
	}
}


func HandlerMsg(s ircx.Sender, m *irc.Message) {
	// silence any errors :)
	//	defer func() {
	//		if r := recover(); r != nil {
	//			lerror.Println("FATAL ERROR in HandleMessage: ", r)
	//		}
	//	}()
	msg := m.Trailing
	sender := m.Prefix.Name
	sender_user := m.Prefix.User
	channel := m.Params[0]

	// Is it a private query?
	if isMe(channel) {  // private msg to the bot
		HandleBotCommand(channel, sender, sender_user, msg)
	} else {  // message on a channel

		// Is someone talking to the bot?
		true_nick := bot.OriginalName
		if strings.HasPrefix(msg, true_nick) {
			msg = strings.TrimLeft(msg[len(true_nick):], ",:; ")
			HandleBotCommand(channel, sender, sender_user, msg)
		}

		// Look for urls
		HandleURLs(channel, sender, msg)
	}

}