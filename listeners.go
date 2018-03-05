package papaBot

// Full message handling routines.

import (
	"fmt"
	"github.com/pawelszydlo/papa-bot/events"
	"github.com/pawelszydlo/papa-bot/utils"
	"log"
	"mvdan.cc/xurls"
	"os"
	"regexp"
	"time"
)

var titleRe = regexp.MustCompile("(?is)<title.*?>(.+?)</title>")
var metaRe = regexp.MustCompile(`(?is)<\s*?meta.*?content\s*?=\s*?"(.*?)".*?>`)
var descRe = regexp.MustCompile(`(?is)(property|name)\s*?=.*?description`)

// messageListener looks for commands in messages.
func (bot *Bot) messageListener(message events.EventMessage) {
	// Increase lines count for all announcements.
	for k := range bot.lastURLAnnouncedLinesPassed {
		bot.lastURLAnnouncedLinesPassed[k] += 1
		// After 100 lines pass, forget it ever happened.
		if bot.lastURLAnnouncedLinesPassed[k] > 100 {
			delete(bot.lastURLAnnouncedLinesPassed, k)
			delete(bot.lastURLAnnouncedTime, k)
		}
	}

	// Handles the commands.
	if message.AtBot {
		bot.handleBotCommand(&message)
	}
}

// handleURLsListener finds all URLs in the message and fires URL events on them.
func (bot *Bot) handleURLsListener(message events.EventMessage) {

	// Find all URLs in the message.
	links := xurls.Strict().FindAllString(message.Message, -1)
	// Remove multiple same links from one message.
	links = utils.RemoveDuplicates(links)
	for i := range links {
		// Validate the url.
		bot.Log.Infof("Got link %s", links[i])
		link := utils.StandardizeURL(links[i])
		bot.Log.Debugf("Standardized to: %s", link)

		// Try to get the body of the page.
		err, finalLink, body := bot.GetPageBody(link, map[string]string{})
		if err != nil {
			bot.Log.Warningf("Could't fetch the body: %s", err)
		}

		// Iterate over meta tags to get the description
		description := ""
		metas := metaRe.FindAllStringSubmatch(string(body), -1)
		for i := range metas {
			if len(metas[i]) > 1 {
				isDesc := descRe.FindString(metas[i][0])
				if isDesc != "" && (len(metas[i][1]) > len(description)) {
					description = utils.CleanString(metas[i][1], true)
				}
			}
		}
		// Get the title
		title := ""
		match := titleRe.FindStringSubmatch(string(body))
		if len(match) > 1 {
			title = utils.CleanString(match[1], true)
		}

		// Insert URL into the db.
		if _, err := bot.Db.Exec(`INSERT INTO urls(transport, channel, nick, link, quote, title) VALUES(?, ?, ?, ?, ?, ?)`,
			message.Transport, message.Channel, message.Nick, finalLink, message.Message, title); err != nil {
			bot.Log.Warningf("Can't add url to database: %s", err)
		}

		// Trigger url found message.
		bot.EventDispatcher.Trigger(events.EventMessage{
			message.Transport,
			events.EventURLFound,
			message.Nick,
			message.UserId,
			message.Channel,
			finalLink,
			message.Context,
			message.AtBot,
		})

		linkKey := finalLink + message.Channel
		// If we can't announce yet, skip this link.
		if time.Since(bot.lastURLAnnouncedTime[linkKey]) < bot.Config.UrlAnnounceIntervalMinutes*time.Minute {
			continue
		}
		if lines, exists := bot.lastURLAnnouncedLinesPassed[linkKey]; exists && lines < bot.Config.UrlAnnounceIntervalLines {
			continue
		}

		// On mattermost we can skip all link info display.
		if message.Transport == "mattermost" {
			return
		}

		// Announce the title, save the description.
		if title != "" {
			if description != "" {
				bot.SendNotice(&message, title+" …")
			} else {
				bot.SendNotice(&message, title)
			}
			bot.lastURLAnnouncedTime[linkKey] = time.Now()
			bot.lastURLAnnouncedLinesPassed[linkKey] = 0
			// Keep the long info for later.
			bot.AddMoreInfo(message.Transport, message.Channel, description)
		}
	}
}

// scribe saves the message into appropriate channel log file.
func (bot *Bot) scribeListener(message events.EventMessage) {
	if !bot.Config.ChatLogging {
		return
	}
	go func() {
		logFileName := fmt.Sprintf(
			"logs/%s_%s_%s.txt", message.Transport, message.Channel, time.Now().Format("2006-01-02"))
		f, err := os.OpenFile(logFileName, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			bot.Log.Errorf("Error opening log file: %s", err)
			return
		}
		defer f.Close()

		scribe := log.New(f, "", log.Ldate|log.Ltime)
		if message.EventCode == events.EventChatMessage {
			scribe.Println(fmt.Sprintf("%s: %s", message.Nick, message.Message))
		} else if message.EventCode == events.EventChatNotice {
			scribe.Println(fmt.Sprintf("Notice from %s: %s", message.Nick, message.Message))
		} else if message.EventCode == events.EventJoinedChannel {
			scribe.Println(fmt.Sprintf("* %s joined.", message.Nick))
		} else if message.EventCode == events.EventPartChannel {
			scribe.Println(fmt.Sprintf("* %s left.", message.Nick))
		} else { // Must be channel activity.
			scribe.Println(fmt.Sprintf("* %s %s", message.Nick, message.Message))
		}
	}()
}
