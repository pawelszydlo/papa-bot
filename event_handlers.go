package papaBot

// Handlers for IRC events.

import (
	"fmt"
	"github.com/mvdan/xurls"
	"github.com/pawelszydlo/papa-bot/utils"
	"github.com/sorcix/irc"
	"math/rand"
	"strings"
	"time"
)

// assignEventHandlers assigns appropriate event handlers.
func (bot *Bot) assignEventHandlers() {
	// Connected to server
	bot.RegisterIrcEventHandler(irc.RPL_WELCOME, handlerConnect)
	// Ping
	bot.RegisterIrcEventHandler(irc.PING, handlerPing)
	// Nickname taken
	bot.RegisterIrcEventHandler(irc.ERR_NICKCOLLISION, handlerNickTaken)
	bot.RegisterIrcEventHandler(irc.ERR_NICKNAMEINUSE, handlerNickTaken)
	// Invalid nickname
	bot.RegisterIrcEventHandler(irc.ERR_NONICKNAMEGIVEN, handlerBadNick)
	bot.RegisterIrcEventHandler(irc.ERR_ERRONEUSNICKNAME, handlerBadNick)
	// Various events that prevent the bot from joining a channel
	bot.RegisterIrcEventHandler(irc.ERR_CHANNELISFULL, handlerCantJoin)
	bot.RegisterIrcEventHandler(irc.ERR_BANNEDFROMCHAN, handlerCantJoin)
	bot.RegisterIrcEventHandler(irc.ERR_INVITEONLYCHAN, handlerCantJoin)
	// Join channel
	bot.RegisterIrcEventHandler(irc.JOIN, handlerJoin)
	// Part channel
	bot.RegisterIrcEventHandler(irc.PART, handlerPart)
	// Set mode
	bot.RegisterIrcEventHandler(irc.MODE, handlerMode)
	// Set topic
	bot.RegisterIrcEventHandler(irc.TOPIC, handlerTopic)
	// Kick from channel
	bot.RegisterIrcEventHandler(irc.KICK, handlerKick)
	// Message on channel
	bot.RegisterIrcEventHandler(irc.PRIVMSG, handlerMsg)
	// Notice
	bot.RegisterIrcEventHandler(irc.NOTICE, handlerDummy)
	// Error
	bot.RegisterIrcEventHandler(irc.ERROR, handlerError)
}

func handlerConnect(bot *Bot, m *irc.Message) {
	bot.Log.Infof("I have connected. Joining channels...")
	bot.SendRawMessage(irc.JOIN, bot.Config.Channels, "")
}

func handlerPing(bot *Bot, m *irc.Message) {
	bot.SendRawMessage(irc.PONG, m.Params, m.Trailing)
}

func handlerNickTaken(bot *Bot, m *irc.Message) {
	bot.Config.Name = bot.Config.Name + "_"
	bot.Log.Warningf(
		"Server at %s said that my nick is already taken. Changing nick to %s", m.Prefix.Name, bot.Config.Name)
}

func handlerCantJoin(bot *Bot, m *irc.Message) {
	bot.Log.Warningf("Server at %s said that I can't join %s: %s", m.Prefix.Name, m.Params[1], m.Trailing)
	// Rejoin
	timer := time.NewTimer(bot.Config.RejoinDelay)
	go func() {
		<-timer.C
		bot.Log.Debugf("Trying to join %s...", m.Params[1])
		bot.SendRawMessage(irc.JOIN, []string{m.Params[1]}, "")
	}()
}

func handlerBadNick(bot *Bot, m *irc.Message) {
	bot.Log.Fatalf("Server at %s said that my nick is invalid.", m.Prefix.Name)
}

func handlerPart(bot *Bot, m *irc.Message) {
	if bot.UserIsMe(m.Prefix.Name) {
		delete(bot.onChannel, m.Params[0])
	}
	bot.Log.Infof("%s has left %s: %s", m.Prefix.Name, m.Params[0], m.Trailing)
	bot.scribe(m.Params[0], m.Prefix.Name, "has left", m.Params[0], ":", m.Trailing)
}

func handlerError(bot *Bot, m *irc.Message) {
	bot.Log.Errorf("Error from server:", m.Trailing)
}

func handlerDummy(bot *Bot, m *irc.Message) {
	bot.Log.Infof("MESSAGE: %+v", m)
}

func handlerJoin(bot *Bot, m *irc.Message) {
	if bot.UserIsMe(m.Prefix.Name) {
		if bot.kickedFrom[m.Trailing] {
			bot.Log.Infof("I have rejoined %s", m.Trailing)
			bot.SendPrivMessage(m.Trailing, bot.Texts.HellosAfterKick[rand.Intn(len(bot.Texts.HellosAfterKick))])
			delete(bot.kickedFrom, m.Trailing)
		} else {
			bot.Log.Infof("I have joined %s", m.Trailing)
			bot.SendPrivMessage(m.Trailing, bot.Texts.Hellos[rand.Intn(len(bot.Texts.Hellos))])
		}
		bot.onChannel[m.Trailing] = true
	} else {
		bot.Log.Infof("%s has joined %s", m.Prefix.Name, m.Trailing)
	}
	bot.scribe(m.Trailing, m.Prefix.Name, " has joined ", m.Trailing)
}

func handlerMode(bot *Bot, m *irc.Message) {
	bot.Log.Infof("%s has set mode %s on %s", m.Prefix.Name, m.Params[1:], m.Params[0])
	bot.scribe(m.Params[0], m.Prefix.Name, "has set mode", m.Params[1:], "on", m.Params[0])
}

func handlerTopic(bot *Bot, m *irc.Message) {
	bot.Log.Infof("%s has set topic on %s to: %s", m.Prefix.Name, m.Params[0], m.Trailing)
	bot.scribe(m.Params[0], m.Prefix.Name, "has set topic on", m.Params[0], "to:", m.Trailing)
}

func handlerKick(bot *Bot, m *irc.Message) {
	if bot.UserIsMe(m.Params[1]) {
		bot.Log.Infof("I was kicked from %s by %s for: %s", m.Params[0], m.Prefix.Name, m.Trailing)
		bot.kickedFrom[m.Params[0]] = true
		delete(bot.onChannel, m.Params[0])
		// Rejoin
		timer := time.NewTimer(bot.Config.RejoinDelay)
		go func() {
			<-timer.C
			bot.SendRawMessage(irc.JOIN, []string{m.Params[0]}, "")
		}()
	} else {
		bot.Log.Infof("%s was kicked from %s by %s for: %s", m.Params[1], m.Prefix.Name, m.Params[0], m.Trailing)
	}
	bot.scribe(m.Params[0], m.Prefix.Name, "has kicked", m.Params[1], "from", m.Params[0], "for:", m.Trailing)
}

func handlerMsg(bot *Bot, m *irc.Message) {
	msg := m.Trailing
	if msg == "" {
		return
	}
	nick := m.Prefix.Name
	user := m.Prefix.User + "@" + m.Prefix.Host
	channel := m.Params[0]

	if bot.UserIsMe(nick) { // It's the bot talking
		return
	}

	// Special CTCP
	if strings.HasPrefix(msg, "\x01") && strings.HasSuffix(msg, "\x01") {
		msg := msg[1 : len(msg)-1]

		if msg == "VERSION" {
			bot.Log.Debugf("Replying to VERSION query from %s...", nick)
			bot.SendNotice(nick, fmt.Sprintf("\x01VERSION papaBot:%s:Go bot running on insomnia.\x01", Version))
			return
		}

		if msg == "FINGER" {
			bot.Log.Debugf("Replying to FINGER query from %s...", nick)
			bot.SendNotice(nick, "\x01FINGER yourself.\x01")
			return
		}

		bot.Log.Debugf("%s sent a %s CTCP request. Ignoring.", nick, msg)
		return
	}

	// Message on a channel.
	bot.scribe(channel, fmt.Sprintf("<%s> %s", nick, msg))

	// Is someone talking to the bot?
	if strings.HasPrefix(msg, bot.Config.Name) {
		msg = strings.TrimLeft(msg[len(bot.Config.Name):], ",:; ")
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
	go bot.handleURLs(channel, nick, msg)

	// Run full message processors.
	go bot.handleMessages(channel, nick, msg)
}

// handlerMsgURLs finds all URLs in the message and executes the URL processors on them.
func (bot *Bot) handleURLs(channel, nick, msg string) {
	currentExtension := ""
	// Catch errors.
	defer func() {
		if Debug {
			return
		} // When in debug mode fail on all errors.
		if r := recover(); r != nil {
			bot.Log.WithField("ext", currentExtension).Errorf("FATAL ERROR in URL processor: %s", r)
		}
	}()

	// Find all URLs in the message.
	links := xurls.Strict.FindAllString(msg, -1)
	// Remove multiple same links from one message.
	links = utils.RemoveDuplicates(links)
	for i := range links {
		// Validate the url.
		bot.Log.Infof("Got link %s", links[i])
		link := utils.StandardizeURL(links[i])
		bot.Log.Debugf("Standardized to: %s", link)
		// Link info structure, it will be filled by the processors.
		var urlinfo UrlInfo
		urlinfo.URL = link
		// Try to get the body of the page.
		if err := bot.GetPageBody(&urlinfo, map[string]string{}); err != nil {
			bot.Log.Warningf("Could't fetch the body: %s", err)
		}

		// Run the extensions.
		for i := range bot.extensions {
			currentExtension = fmt.Sprintf("%T", bot.extensions[i])
			bot.extensions[i].ProcessURL(bot, channel, nick, msg, &urlinfo)
		}

		// Insert URL into the db.
		if _, err := bot.Db.Exec(`INSERT INTO urls(channel, nick, link, quote, title) VALUES(?, ?, ?, ?, ?)`,
			channel, nick, urlinfo.URL, msg, urlinfo.Title); err != nil {
			bot.Log.Warningf("Can't add url to database: %s", err)
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
func (bot *Bot) handleMessages(channel, nick, msg string) {
	currentExtension := ""
	// Catch errors.
	defer func() {
		if Debug {
			return
		} // When in debug mode fail on all errors.
		if r := recover(); r != nil {
			bot.Log.WithField("ext", currentExtension).Errorf("FATAL ERROR in msg processor: %s", r)
		}
	}()

	// Run the extensions.
	for i := range bot.extensions {
		currentExtension = fmt.Sprintf("%T", bot.extensions[i])
		bot.extensions[i].ProcessMessage(bot, channel, nick, msg)
	}
}
