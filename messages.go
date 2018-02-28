package papaBot

// Full message handling routines.

import (
	"fmt"
	"github.com/mvdan/xurls"
	"github.com/pawelszydlo/papa-bot/transports"
	"github.com/pawelszydlo/papa-bot/utils"
	"log"
	"os"
	"time"
)

// handleMessage handles processing of normal messages, like: logging, finding URLs and extensions.
func (bot *Bot) handleMessage(transport string, message transports.ScribeMessage) {
	// Check if sender is not ignored.
	if bot.isNickIgnored(transport, message.Who) {
		bot.Log.Debugf("Ignoring what %s says.", message.Who)
		return
	}

	// Log.
	bot.scribe(transport, message)

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
	go bot.handleURLs(transport, message.Where, message.Who, message.What)

	// Run full message processors.
	go bot.processMessage(transport, message.Where, message.Who, message.What)
}

// handlerMsgURLs finds all URLs in the message and executes the URL processors on them.
func (bot *Bot) handleURLs(transport, channel, nick, msg string) {
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
			bot.extensions[i].ProcessURL(bot, transport, channel, nick, msg, &urlinfo)
		}

		// Insert URL into the db.
		if _, err := bot.Db.Exec(`INSERT INTO urls(transport, channel, nick, link, quote, title) VALUES(?, ?, ?, ?, ?, ?)`,
			transport, channel, nick, urlinfo.URL, msg, urlinfo.Title); err != nil {
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
				bot.SendNotice(transport, channel, urlinfo.ShortInfo+" …")
			} else {
				bot.SendNotice(transport, channel, urlinfo.ShortInfo)
			}
			bot.lastURLAnnouncedTime[linkKey] = time.Now()
			bot.lastURLAnnouncedLinesPassed[linkKey] = 0
			// Keep the long info for later.
			bot.urlMoreInfo[transport+channel] = urlinfo.LongInfo
		}
	}
}

// processMessage runs the processors on the whole message.
func (bot *Bot) processMessage(transport, channel, nick, msg string) {
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
		bot.extensions[i].ProcessMessage(bot, transport, channel, nick, msg)
	}
}

// scribe saves the message into appropriate channel log file.
func (bot *Bot) scribe(transportName string, message transports.ScribeMessage) {
	if !bot.Config.ChatLogging {
		return
	}
	go func() {
		logFileName := fmt.Sprintf("logs/%s_%s_%s.txt", transportName, message.Where, time.Now().Format("2006-01-02"))
		f, err := os.OpenFile(logFileName, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			bot.Log.Errorf("Error opening log file: %s", err)
			return
		}
		defer f.Close()

		scribe := log.New(f, "", log.Ldate|log.Ltime)
		if message.Special {
			scribe.Println(fmt.Sprintf("* %s %s", message.Who, message.What))
		} else {
			scribe.Println(fmt.Sprintf("%s: %s", message.Who, message.What))
		}
	}()
}
