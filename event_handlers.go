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
	bot.eventHandlers[irc.RPL_WELCOME] = bot.handlerConnect
	// Ping
	bot.eventHandlers[irc.PING] = bot.handlerPing
	// Nickname taken
	bot.eventHandlers[irc.ERR_NICKCOLLISION] = bot.handlerNickTaken
	bot.eventHandlers[irc.ERR_NICKNAMEINUSE] = bot.handlerNickTaken
	// Invalid nickname
	bot.eventHandlers[irc.ERR_NONICKNAMEGIVEN] = bot.handlerBadNick
	bot.eventHandlers[irc.ERR_ERRONEUSNICKNAME] = bot.handlerBadNick
	// Various events that prevent the bot from joining a channel
	bot.eventHandlers[irc.ERR_CHANNELISFULL] = bot.handlerCantJoin
	bot.eventHandlers[irc.ERR_BANNEDFROMCHAN] = bot.handlerCantJoin
	bot.eventHandlers[irc.ERR_INVITEONLYCHAN] = bot.handlerCantJoin
	// Join channel
	bot.eventHandlers[irc.JOIN] = bot.handlerJoin
	// Part channel
	bot.eventHandlers[irc.PART] = bot.handlerPart
	// Set mode
	bot.eventHandlers[irc.MODE] = bot.handlerMode
	// Set topic
	bot.eventHandlers[irc.TOPIC] = bot.handlerTopic
	// Kick from channel
	bot.eventHandlers[irc.KICK] = bot.handlerKick
	// Message on channel
	bot.eventHandlers[irc.PRIVMSG] = bot.handlerMsg
	// Notice
	bot.eventHandlers[irc.NOTICE] = bot.handlerDummy
	// Error
	bot.eventHandlers[irc.ERROR] = bot.handlerError
}

func (bot *Bot) handlerConnect(m *irc.Message) {
	bot.Log.Info("I have connected. Joining channels...")
	bot.SendRawMessage(irc.JOIN, bot.Config.Channels, "")
}

func (bot *Bot) handlerPing(m *irc.Message) {
	bot.SendRawMessage(irc.PONG, m.Params, m.Trailing)
}

func (bot *Bot) handlerNickTaken(m *irc.Message) {
	bot.Config.Name = bot.Config.Name + "_"
	bot.Log.Warning(
		"Server at %s said that my nick is already taken. Changing nick to %s", m.Prefix.Name, bot.Config.Name)
}

func (bot *Bot) handlerCantJoin(m *irc.Message) {
	bot.Log.Warning("Server at %s said that I can't join %s: %s", m.Prefix.Name, m.Params[1], m.Trailing)
	// Rejoin
	timer := time.NewTimer(bot.Config.RejoinDelay)
	go func() {
		<-timer.C
		bot.Log.Debug("Trying to join %s...", m.Params[1])
		bot.SendRawMessage(irc.JOIN, []string{m.Params[1]}, "")
	}()
}

func (bot *Bot) handlerBadNick(m *irc.Message) {
	bot.Log.Fatalf("Server at %s said that my nick is invalid.", m.Prefix.Name)
}

func (bot *Bot) handlerPart(m *irc.Message) {
	if bot.UserIsMe(m.Prefix.Name) {
		delete(bot.onChannel, m.Params[0])
	}
	bot.Log.Info("%s has left %s: %s", m.Prefix.Name, m.Params[0], m.Trailing)
	bot.scribe(m.Params[0], m.Prefix.Name, "has left", m.Params[0], ":", m.Trailing)
}

func (bot *Bot) handlerError(m *irc.Message) {
	bot.Log.Error("Error from server:", m.Trailing)
}

func (bot *Bot) handlerDummy(m *irc.Message) {
	bot.Log.Info("MESSAGE: %+v", m)
}

func (bot *Bot) handlerJoin(m *irc.Message) {
	if bot.UserIsMe(m.Prefix.Name) {
		if bot.kickedFrom[m.Trailing] {
			bot.Log.Info("I have rejoined %s", m.Trailing)
			bot.SendPrivMessage(m.Trailing, bot.Texts.HellosAfterKick[rand.Intn(len(bot.Texts.HellosAfterKick))])
			delete(bot.kickedFrom, m.Trailing)
		} else {
			bot.Log.Info("I have joined %s", m.Trailing)
			bot.SendPrivMessage(m.Trailing, bot.Texts.Hellos[rand.Intn(len(bot.Texts.Hellos))])
		}
		bot.onChannel[m.Trailing] = true
	} else {
		bot.Log.Info("%s has joined %s", m.Prefix.Name, m.Trailing)
	}
	bot.scribe(m.Trailing, m.Prefix.Name, " has joined ", m.Trailing)
}

func (bot *Bot) handlerMode(m *irc.Message) {
	bot.Log.Info("%s has set mode %s on %s", m.Prefix.Name, m.Params[1:], m.Params[0])
	bot.scribe(m.Params[0], m.Prefix.Name, "has set mode", m.Params[1:], "on", m.Params[0])
}

func (bot *Bot) handlerTopic(m *irc.Message) {
	bot.Log.Info("%s has set topic on %s to: %s", m.Prefix.Name, m.Params[0], m.Trailing)
	bot.scribe(m.Params[0], m.Prefix.Name, "has set topic on", m.Params[0], "to:", m.Trailing)
}

func (bot *Bot) handlerKick(m *irc.Message) {
	if bot.UserIsMe(m.Params[1]) {
		bot.Log.Info("I was kicked from %s by %s for: %s", m.Prefix.Name, m.Params[0], m.Trailing)
		bot.kickedFrom[m.Params[0]] = true
		delete(bot.onChannel, m.Params[0])
		// Rejoin
		timer := time.NewTimer(bot.Config.RejoinDelay)
		go func() {
			<-timer.C
			bot.SendRawMessage(irc.JOIN, []string{m.Params[0]}, "")
		}()
	} else {
		bot.Log.Info("%s was kicked from %s by %s for: %s", m.Params[1], m.Prefix.Name, m.Params[0], m.Trailing)
	}
	bot.scribe(m.Params[0], m.Prefix.Name, "has kicked", m.Params[1], "from", m.Params[0], "for:", m.Trailing)
}

func (bot *Bot) handlerMsg(m *irc.Message) {
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
			bot.Log.Debug("Replying to VERSION query from %s...", nick)
			bot.SendNotice(nick, fmt.Sprintf("\x01VERSION papaBot:%s:Go bot running on insomnia.\x01", Version))
			return
		}

		if msg == "FINGER" {
			bot.Log.Debug("Replying to FINGER query from %s...", nick)
			bot.SendNotice(nick, "\x01FINGER yourself.\x01")
			return
		}

		bot.Log.Debug("%s sent a %s CTCP request. Ignoring.", nick, msg)
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
			bot.Log.Error("FATAL ERROR in URL processor: %s", r)
		}
	}()

	// Find all URLs in the message.
	links := xurls.Relaxed.FindAllString(msg, -1)
	// Remove multiple same links from one message.
	links = utils.RemoveDuplicates(links)
	for i := range links {
		// Validate the url.
		bot.Log.Info("Got link %s", links[i])
		link := utils.StandardizeURL(links[i])
		bot.Log.Debug("Standardized to: %s", link)
		// Link info structure, it will be filled by the processors.
		var urlinfo UrlInfo
		urlinfo.URL = link
		// Try to get the body of the page.
		if err := bot.GetPageBody(&urlinfo, map[string]string{}); err != nil {
			bot.Log.Warning("Could't fetch the body: %s", err)
		}

		// Run the extensions.
		for i := range bot.extensions {
			bot.extensions[i].ProcessURL(bot, &urlinfo, channel, nick, msg)
		}

		// Insert URL into the db.
		if _, err := bot.Db.Exec(`INSERT INTO urls(channel, nick, link, quote, title) VALUES(?, ?, ?, ?, ?)`,
			channel, nick, urlinfo.URL, msg, urlinfo.Title); err != nil {
			bot.Log.Warning("Can't add url to database: %s", err)
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
			bot.Log.Error("FATAL ERROR in msg processor: %s", r)
		}
	}()

	// Run the extensions.
	for i := range bot.extensions {
		bot.extensions[i].ProcessMessage(bot, channel, nick, msg)
	}
}
