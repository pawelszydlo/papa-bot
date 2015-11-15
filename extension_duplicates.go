package papaBot

import (
	"fmt"
	"github.com/pawelszydlo/papa-bot/utils"
	"text/template"
	"time"
)

// ExtensionDuplicates checks for duplicate URLs posted.
type ExtensionDuplicates struct {
	Extension
	Texts     *ExtensionDuplicatesTexts
	announced map[string]time.Time
}

type ExtensionDuplicatesTexts struct {
	TempDuplicateFirst *template.Template
	TplDuplicateFirst  string

	TempDuplicateMulti *template.Template
	TplDuplicateMulti  string

	DuplicateYou string
}

// Init inits the extension.
func (ext *ExtensionDuplicates) Init(bot *Bot) error {
	texts := new(ExtensionDuplicatesTexts) // Can't load directly because of reflection issues.
	if err := bot.LoadTexts(bot.textsFile, texts); err != nil {
		return err
	}
	ext.Texts = texts
	ext.announced = map[string]time.Time{}
	return nil
}

// checkForDuplicates checks for duplicates of the url in the database.
func (ext *ExtensionDuplicates) ProcessURL(bot *Bot, urlinfo *UrlInfo, channel, sender, msg string) {
	result, err := bot.Db.Query(`
		SELECT IFNULL(nick, ""), IFNULL(timestamp, datetime('now')), count(*)
		FROM urls WHERE link=? AND channel=?
		ORDER BY timestamp DESC LIMIT 1`, urlinfo.URL, channel)
	if err != nil {
		bot.log.Warning("Can't query the database for duplicates: %s", err)
		return
	}
	defer result.Close()

	// Announce a duplicate
	if result.Next() {
		var nick string
		var timestr string
		var count uint
		if err = result.Scan(&nick, &timestr, &count); err != nil {
			bot.log.Warning("Error getting duplicates: %s", err)
			return
		}
		timestamp, _ := time.Parse("2006-01-02 15:04:05", timestr)
		duplicate := ""
		// Only one duplicate
		if count == 1 {
			if bot.areSamePeople(nick, sender) {
				nick = ext.Texts.DuplicateYou
			}
			elapsed := HumanizedSince(utils.MustForceLocalTimezone(timestamp))
			duplicate = utils.Format(ext.Texts.TempDuplicateFirst, map[string]string{"nick": nick, "elapsed": elapsed})
		} else if count > 1 { // More duplicates exist
			if bot.areSamePeople(nick, sender) {
				nick = ext.Texts.DuplicateYou
			}
			elapsed := HumanizedSince(utils.MustForceLocalTimezone(timestamp))
			duplicate = utils.Format(ext.Texts.TempDuplicateMulti,
				map[string]string{"nick": nick, "elapsed": elapsed, "count": fmt.Sprintf("%d", count)})
		}
		// Only announce once per 5 minutes per link.
		if duplicate != "" && time.Since(ext.announced[channel+urlinfo.URL]) > 5*time.Minute {
			// Can we fit into the ShortInfo?
			if urlinfo.ShortInfo == "" {
				urlinfo.ShortInfo = duplicate
			} else if len(urlinfo.ShortInfo) < 50 {
				urlinfo.ShortInfo += " | " + duplicate
			} else { // Better send as separate noitce.
				bot.SendNotice(channel, duplicate)
			}
			ext.announced[channel+urlinfo.URL] = time.Now()
		}
	}
	return
}

// Tick will clean announces table once per day.
func (ext *ExtensionDuplicates) Tick(bot *Bot, daily bool) {
	if daily {
		ext.announced = map[string]time.Time{}
	}
}
