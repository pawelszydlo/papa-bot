package papaBot

// Handlers for IRC events.

import (
	"fmt"
	"github.com/mvdan/xurls"
	"github.com/nickvanw/ircx"
	"github.com/pawelszydlo/papa-bot/utils"
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
	// Nickname taken
	bot.irc.HandleFunc(irc.ERR_NICKCOLLISION, bot.handlerNickTaken)
	bot.irc.HandleFunc(irc.ERR_NICKNAMEINUSE, bot.handlerNickTaken)
	// Invalid nickname
	bot.irc.HandleFunc(irc.ERR_NONICKNAMEGIVEN, bot.handlerBadNick)
	bot.irc.HandleFunc(irc.ERR_ERRONEUSNICKNAME, bot.handlerBadNick)
	// Various events that prevent the bot from joining a channel
	bot.irc.HandleFunc(irc.ERR_CHANNELISFULL, bot.handlerCantJoin)
	bot.irc.HandleFunc(irc.ERR_BANNEDFROMCHAN, bot.handlerCantJoin)
	bot.irc.HandleFunc(irc.ERR_INVITEONLYCHAN, bot.handlerCantJoin)
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
	bot.log.Info("I have connected. Joining channels...")
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

func (bot *Bot) handlerNickTaken(s ircx.Sender, m *irc.Message) {
	bot.irc.OriginalName = bot.Config.Name + "_"
	bot.log.Warning(
		"Server at %s said that my nick is already taken. Changing nick to %s", m.Prefix.Name, bot.irc.OriginalName)
}

func (bot *Bot) handlerCantJoin(s ircx.Sender, m *irc.Message) {
	bot.log.Warning("Server at %s said that I can't join %s: %s", m.Prefix.Name, m.Params[1], m.Trailing)
	// Rejoin
	timer := time.NewTimer(bot.Config.RejoinDelaySeconds * time.Second)
	go func() {
		<-timer.C
		bot.log.Debug("Trying to join %s...", m.Params[1])
		s.Send(&irc.Message{
			Command: irc.JOIN,
			Params:  []string{m.Params[1]},
		})
	}()
}

func (bot *Bot) handlerBadNick(s ircx.Sender, m *irc.Message) {
	bot.log.Fatal("Server at %s said that my nick is invalid.", m.Prefix.Name)
}

func (bot *Bot) handlerPart(s ircx.Sender, m *irc.Message) {
	if bot.userIsMe(m.Prefix.Name) {
		delete(bot.onChannel, m.Params[0])
	}
	bot.log.Info("%s has left %s: %s", m.Prefix.Name, m.Params[0], m.Trailing)
	bot.scribe(m.Params[0], m.Prefix.Name, "has left", m.Params[0], ":", m.Trailing)
}

func (bot *Bot) handlerError(s ircx.Sender, m *irc.Message) {
	bot.log.Error("Error from server:", m.Trailing)
}

func (bot *Bot) handlerDummy(s ircx.Sender, m *irc.Message) {
	bot.log.Info("MESSAGE: %+v", m)
}

func (bot *Bot) handlerJoin(s ircx.Sender, m *irc.Message) {
	if bot.userIsMe(m.Prefix.Name) {
		if bot.kickedFrom[m.Trailing] {
			bot.log.Info("I have rejoined %s", m.Trailing)
			bot.SendMessage(m.Trailing, bot.Texts.HellosAfterKick[rand.Intn(len(bot.Texts.HellosAfterKick))])
			delete(bot.kickedFrom, m.Trailing)
		} else {
			bot.log.Info("I have joined %s", m.Trailing)
			bot.SendMessage(m.Trailing, bot.Texts.Hellos[rand.Intn(len(bot.Texts.Hellos))])
		}
		bot.onChannel[m.Trailing] = true
	} else {
		bot.log.Info("%s has joined %s", m.Prefix.Name, m.Trailing)
	}
	bot.scribe(m.Trailing, m.Prefix.Name, " has joined ", m.Trailing)
}

func (bot *Bot) handlerMode(s ircx.Sender, m *irc.Message) {
	bot.log.Info("%s has set mode %s on %s", m.Prefix.Name, m.Params[1:], m.Params[0])
	bot.scribe(m.Params[0], m.Prefix.Name, "has set mode", m.Params[1:], "on", m.Params[0])
}

func (bot *Bot) handlerTopic(s ircx.Sender, m *irc.Message) {
	bot.log.Info("%s has set topic on %s to: %s", m.Prefix.Name, m.Params[0], m.Trailing)
	bot.scribe(m.Params[0], m.Prefix.Name, "has set topic on", m.Params[0], "to:", m.Trailing)
}

func (bot *Bot) handlerKick(s ircx.Sender, m *irc.Message) {
	if bot.userIsMe(m.Params[1]) {
		bot.log.Info("I was kicked from %s by %s for: %s", m.Prefix.Name, m.Params[0], m.Trailing)
		bot.kickedFrom[m.Params[0]] = true
		delete(bot.onChannel, m.Params[0])
		// Rejoin
		timer := time.NewTimer(bot.Config.RejoinDelaySeconds * time.Second)
		go func() {
			<-timer.C
			s.Send(&irc.Message{
				Command: irc.JOIN,
				Params:  []string{m.Params[0]},
			})
		}()
	} else {
		bot.log.Info("%s was kicked from %s by %s for: %s", m.Params[1], m.Prefix.Name, m.Params[0], m.Trailing)
	}
	bot.scribe(m.Params[0], m.Prefix.Name, "has kicked", m.Params[1], "from", m.Params[0], "for:", m.Trailing)
}

func (bot *Bot) handlerMsg(s ircx.Sender, m *irc.Message) {
	msg := m.Trailing
	if msg == "" {
		return
	}
	nick := m.Prefix.Name
	user := m.Prefix.User + "@" + m.Prefix.Host
	channel := m.Params[0]

	if bot.userIsMe(nick) { // It's the bot talking
		return
	}

	// Special CTCP
	if strings.HasPrefix(msg, "\x01") && strings.HasSuffix(msg, "\x01") {
		msg := msg[1 : len(msg)-1]

		if msg == "VERSION" {
			bot.log.Debug("Replying to VERSION query from %s...", nick)
			bot.SendNotice(nick, fmt.Sprintf("\x01VERSION papaBot:%s:Go bot running on insomnia.\x01", Version))
			return
		}

		if msg == "FINGER" {
			bot.log.Debug("Replying to FINGER query from %s...", nick)
			bot.SendNotice(nick, "\x01FINGER yourself.\x01")
			return
		}

		bot.log.Debug("%s sent a %s CTCP request. Ignoring.", nick, msg)
		return
	}

	// Message on a channel.
	bot.scribe(channel, fmt.Sprintf("<%s> %s", nick, msg))

	// Is someone talking to the bot?
	true_nick := bot.irc.OriginalName
	if strings.HasPrefix(msg, true_nick) {
		msg = strings.TrimLeft(msg[len(true_nick):], ",:; ")
		if msg != "" {
			go bot.handleBotCommand(channel, nick, user, msg, true)
			return
		}
	}

	// Maybe a dot command?
	if strings.HasPrefix(msg, ".") {
		msg = strings.TrimPrefix(msg, ".")
		if msg != "" {
			go bot.handleBotCommand(channel, nick, user, msg, false)
			return
		}
	}

	// Increase lines count for all announcements.
	for k := range bot.lastURLAnnouncedLinesPassed {
		bot.lastURLAnnouncedLinesPassed[k] += 1
		// After 100 lines pass, forget it ever happened.
		if bot.lastURLAnnouncedLinesPassed[k] > 100 {
			delete(bot.lastURLAnnouncedLinesPassed, k)
			delete(bot.lastURLAnnouncedTime, k)
		}
	}

	// Handle links in the message.
	go bot.handlerMsgURLs(channel, nick, msg)

	// Run full message processors.
	go bot.handlerMsgFull(channel, nick, msg)
}

// handlerMsgURLs finds all URLs in the message and executes the URL processors on them.
func (bot *Bot) handlerMsgURLs(channel, nick, msg string) {
	// Catch errors.
	defer func() {
		if Debug {
			return
		} // When in debug mode fail on all errors.
		if r := recover(); r != nil {
			bot.log.Error("FATAL ERROR in URL processor: %s", r)
		}
	}()

	// Find all URLs in the message.
	links := xurls.Relaxed.FindAllString(msg, -1)
	// Remove multiple same links from one message.
	links = utils.RemoveDuplicates(links)
	for i := range links {
		// Validate the url.
		bot.log.Info("Got link %s", links[i])
		link := utils.StandardizeURL(links[i])
		bot.log.Debug("Standardized to: %s", link)
		// Link info structure, it will be filled by the processors.
		var urlinfo UrlInfo
		urlinfo.URL = link
		// Try to get the body of the page.
		if err := bot.getPageBody(&urlinfo, map[string]string{}); err != nil {
			bot.log.Warning("Could't fetch the body: %s", err)
		}

		// Run the extensions.
		for i := range bot.extensions {
			bot.extensions[i].ProcessURL(bot, &urlinfo, channel, nick, msg)
		}

		// Insert URL into the db.
		if _, err := bot.Db.Exec(`INSERT INTO urls(channel, nick, link, quote, title) VALUES(?, ?, ?, ?, ?)`,
			channel, nick, urlinfo.URL, msg, urlinfo.Title); err != nil {
			bot.log.Warning("Can't add url to database: %s", err)
		}

		linkKey := urlinfo.URL + channel
		// If we can't announce yet, skip this link.
		if time.Since(bot.lastURLAnnouncedTime[linkKey]) < bot.Config.UrlAnnounceIntervalMinutes*time.Minute {
			continue
		}
		if lines, exists := bot.lastURLAnnouncedLinesPassed[linkKey]; exists && lines < bot.Config.UrlAnnounceIntervalLines {
			continue
		}

		// Announce the short info, save the long info.
		if urlinfo.ShortInfo != "" {
			if urlinfo.LongInfo != "" {
				bot.SendNotice(channel, urlinfo.ShortInfo+" …")
			} else {
				bot.SendNotice(channel, urlinfo.ShortInfo)
			}
			bot.lastURLAnnouncedTime[linkKey] = time.Now()
			bot.lastURLAnnouncedLinesPassed[linkKey] = 0
			// Keep the long info for later.
			bot.urlMoreInfo[channel] = urlinfo.LongInfo
		}
	}
}

// handlerMsgFull runs the processors on the whole message.
func (bot *Bot) handlerMsgFull(channel, nick, msg string) {
	// Catch errors.
	defer func() {
		if Debug {
			return
		} // When in debug mode fail on all errors.
		if r := recover(); r != nil {
			bot.log.Error("FATAL ERROR in msg processor: %s", r)
		}
	}()

	// Run the extensions.
	for i := range bot.extensions {
		bot.extensions[i].ProcessMessage(bot, channel, nick, msg)
	}
}
