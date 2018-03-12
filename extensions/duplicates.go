package extensions

import (
	"fmt"
	"github.com/pawelszydlo/papa-bot"
	"github.com/pawelszydlo/papa-bot/events"
	"github.com/pawelszydlo/papa-bot/utils"
	"text/template"
	"time"
)

// ExtensionDuplicates checks for duplicate URLs posted.
type ExtensionDuplicates struct {
	Texts     *extensionDuplicatesTexts
	announced map[string]time.Time
	bot       *papaBot.Bot
}

type extensionDuplicatesTexts struct {
	TempDuplicateFirst *template.Template
	TempDuplicateMulti *template.Template

	DuplicateYou string
}

// Init inits the extension.
func (ext *ExtensionDuplicates) Init(bot *papaBot.Bot) error {
	texts := new(extensionDuplicatesTexts) // Can't load directly because of reflection issues.
	if err := bot.LoadTexts("duplicates", texts); err != nil {
		return err
	}
	ext.Texts = texts
	ext.announced = map[string]time.Time{}
	ext.bot = bot
	bot.EventDispatcher.RegisterListener(events.EventURLFound, ext.ProcessURLListener)
	return nil
}

// checkForDuplicates checks for duplicates of the url in the database.
func (ext *ExtensionDuplicates) ProcessURLListener(message events.EventMessage) {
	result, err := ext.bot.Db.Query(`
		SELECT IFNULL(nick, ""), IFNULL(timestamp, datetime('now')), count(*)
		FROM urls WHERE link=? AND channel=? AND transport=?
		ORDER BY timestamp DESC LIMIT 1,1`, message.Message, message.Channel, message.TransportName)
	if err != nil {
		ext.bot.Log.Warningf("Can't query the database for duplicates: %s", err)
		return
	}
	defer result.Close()

	// Because bot already recorded this occurrence, we are interested in the one before, hence LIMIT 1,1.
	if result.Next() {
		var nick string
		var timestr string
		var count uint
		if err = result.Scan(&nick, &timestr, &count); err != nil {
			ext.bot.Log.Warningf("Error getting duplicates: %s", err)
			return
		}
		timestamp, _ := time.Parse("2006-01-02 15:04:05", timestr)
		elapsed := ext.bot.Humanizer.TimeDiffNow(utils.MustForceLocalTimezone(timestamp))
		duplicate := ""
		// Only one duplicate.
		if count == 2 {
			if ext.bot.AreSamePeople(nick, message.Nick) {
				nick = ext.Texts.DuplicateYou
			}
			duplicate = utils.Format(ext.Texts.TempDuplicateFirst, map[string]string{"nick": nick, "elapsed": elapsed})
		} else if count > 2 { // More duplicates exist
			if ext.bot.AreSamePeople(nick, message.Nick) {
				nick = ext.Texts.DuplicateYou
			}
			duplicate = utils.Format(ext.Texts.TempDuplicateMulti,
				map[string]string{"nick": nick, "elapsed": elapsed, "count": fmt.Sprintf("%d", count-1)})
		}
		// Only announce once per 5 minutes per link.
		if duplicate != "" && time.Since(ext.announced[message.Channel+message.Message]) > 5*time.Minute {
			ext.bot.SendNotice(&message, duplicate)
			ext.announced[message.Channel+message.Message] = time.Now()
		}
	}
	return
}

// Tick will clean announces table once per day.
func (ext *ExtensionDuplicates) Tick(bot *papaBot.Bot, daily bool) {
	if daily {
		ext.announced = map[string]time.Time{}
	}
}
