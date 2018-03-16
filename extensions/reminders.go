package extensions

import (
	"fmt"
	"github.com/pawelszydlo/papa-bot"
	"github.com/pawelszydlo/papa-bot/events"
	"github.com/pawelszydlo/papa-bot/utils"
	"strconv"
	"strings"
	"text/template"
	"time"
)

// ExtensionReminders - enables the creation of custom reminders.
type ExtensionReminders struct {
	reminders map[int]*extensionRemindersReminder
	bot       *papaBot.Bot
	texts     *extensionRemindersTexts
}

type extensionRemindersReminder struct {
	transport   string
	channel     string
	creator     string
	text        string
	announced   bool
	createdTime time.Time
	targetTime  time.Time
}

type extensionRemindersTexts struct {
	TempAnnounce *template.Template
}

// Init initializes the extension.
func (ext *ExtensionReminders) Init(bot *papaBot.Bot) error {
	ext.bot = bot
	// Create database table to hold the remainders.
	query := `
		CREATE TABLE IF NOT EXISTS "reminders" (
			"id" INTEGER PRIMARY KEY  AUTOINCREMENT  NOT NULL,
			"transport" VARCHAR NOT NULL,
			"channel" VARCHAR NOT NULL,
			"creator" VARCHAR NOT NULL,
			"announce_text" VARCHAR NOT NULL,
			"announced" INTEGER DEFAULT 0,
			"target_time" VARCHAR NOT NULL,
			"created_time" VARCHAR NOT NULL
		);`
	if _, err := bot.Db.Exec(query); err != nil {
		bot.Log.Panic(err)
	}

	// Load texts.
	texts := &extensionRemindersTexts{}
	if err := bot.LoadTexts("reminders", texts); err != nil {
		return err
	}
	ext.texts = texts

	// Add commands for handling the counters.
	bot.RegisterCommand(&papaBot.BotCommand{
		[]string{"rm", "remind"},
		false, false, false,
		"help / list / del <id> / add <time to wait> <text>",
		"Creates and manages remainders.",
		ext.commandRemind})

	// Load reminders from the db.
	ext.loadReminders()

	// Attach to events.
	bot.EventDispatcher.RegisterListener(events.EventTick, ext.TickListener)

	return nil
}

// announce will announce the reminder.
func (ext *ExtensionReminders) announce(id int) {
	r := ext.reminders[id]
	message := utils.Format(ext.texts.TempAnnounce, map[string]string{
		"who":  r.creator,
		"what": r.text,
		"when": ext.bot.Humanizer.TimeDiffNow(r.createdTime, false),
	})
	sourceEvent := &events.EventMessage{
		r.transport,
		events.FormatPlain,
		events.EventChatMessage,
		r.creator,
		"",
		r.channel,
		"",
		"",
		false,
	}
	ext.bot.SendMessage(sourceEvent, message)

	// Mark as announced in the db.
	query := `UPDATE reminders SET announced = 1 WHERE id=?;`
	if _, err := ext.bot.Db.Exec(query, id); err != nil {
		ext.bot.Log.Errorf("Error while marking reminder as announced: %s", err)
		return
	}

	// Reload reminders.
	ext.loadReminders()
}

// TickListener will trigger the reminder if needed.
func (ext *ExtensionReminders) TickListener(message events.EventMessage) {
	for id, reminder := range ext.reminders {
		if reminder.announced {
			continue
		}
		if time.Now().After(reminder.targetTime) {
			ext.announce(id)
		}
	}
}

// loadReminders will load the reminders from the database.
func (ext *ExtensionReminders) loadReminders() {
	ext.reminders = map[int]*extensionRemindersReminder{}

	result, err := ext.bot.Db.Query(
		`SELECT id, channel, transport, creator, announce_text, announced, target_time, created_time ` +
			`FROM reminders WHERE announced = 0`)
	if err != nil {
		ext.bot.Log.Warningf("Error while loading reminders: %s", err)
		return
	}
	defer result.Close()

	// Get vars.
	for result.Next() {
		var r extensionRemindersReminder
		var createdTimeStr, targetTimeStr string
		var id int
		if err = result.Scan(
			&id, &r.channel, &r.transport, &r.creator, &r.text, &r.announced, &targetTimeStr, &createdTimeStr,
		); err != nil {
			ext.bot.Log.Warningf("Can't load reminder: %s", err)
			continue
		}
		// Handle the dates.
		r.targetTime, err = time.Parse("2006-01-02 15:04:05", targetTimeStr)
		if err != nil {
			ext.bot.Log.Fatalf("Can't parse reminder date %s: %s", targetTimeStr, err)
		}
		r.targetTime = utils.MustForceLocalTimezone(r.targetTime)
		r.createdTime, err = time.Parse("2006-01-02 15:04:05", createdTimeStr)
		if err != nil {
			ext.bot.Log.Fatalf("Can't parse reminder date %s: %s", createdTimeStr, err)
		}
		r.createdTime = utils.MustForceLocalTimezone(r.createdTime)

		ext.reminders[id] = &r
	}
}

// printReminders is a helper function for preparing reminder lists.
func (ext *ExtensionReminders) printReminders(bot *papaBot.Bot, sourceEvent *events.EventMessage, all bool) {
	reminders := []string{}
	for id, reminder := range ext.reminders {
		if !all && (reminder.transport != sourceEvent.TransportName || reminder.channel != sourceEvent.Channel) {
			continue
		}
		if reminder.announced {
			continue
		}
		reminders = append(reminders, fmt.Sprintf(
			"| %d | %s | %s | %s | %s |",
			id,
			bot.Humanizer.TimeDiffNow(reminder.createdTime, false),
			reminder.creator,
			bot.Humanizer.TimeDiffNow(reminder.targetTime, false),
			reminder.text,
		))

	}
	result := ""
	if sourceEvent.TransportFormatting == events.FormatMarkdown {
		result = "\n\n| id | set | by | announce | |\n| -:- | :-- | :-- | :-- | :-- |\n"
	}
	result += strings.Join(reminders, "\n")
	bot.SendMessage(sourceEvent, result)
}

// commandRemind is a command for handling the reminders.
func (ext *ExtensionReminders) commandRemind(bot *papaBot.Bot, sourceEvent *events.EventMessage, params []string) {

	if len(params) < 1 {
		return
	}
	command := params[0]

	// List.
	if command == "list" {
		if len(ext.reminders) > 0 {
			bot.SendMessage(sourceEvent, "Active reminders:")
			// If user is an admin and conversation is private - show all reminders.
			if sourceEvent.IsPrivate() && bot.UserIsOwnerOrAdmin(sourceEvent.UserId) {
				ext.printReminders(bot, sourceEvent, true)
			} else { // If not, show only reminders for this channel.
				ext.printReminders(bot, sourceEvent, false)
			}
		} else {
			bot.SendMessage(sourceEvent, "No reminders yet.")
		}
		return
	}

	if command == "help" {
		bot.SendMessage(sourceEvent, "To add a new reminder: .rm add <time to wait> <text>")
		bot.SendMessage(
			sourceEvent, `Where time to wait is in format "X units", e.g. "5 days" or "2 years". `+
				`Please note that actual announce granularity is 5 minutes.`)
		return
	}

	// Delete.
	if len(params) == 2 && command == "del" {
		id, err := strconv.ParseInt(params[1], 10, 32)
		if err != nil {
			bot.SendMessage(sourceEvent, "id must be a number.")
			return
		}
		reminder, exists := ext.reminders[int(id)]
		if !exists {
			bot.SendMessage(sourceEvent, "Reminder not found.")
			return
		}

		// Is the reminder set for different channel?
		if reminder.transport != sourceEvent.TransportName || reminder.channel != sourceEvent.Channel {
			if !bot.UserIsOwnerOrAdmin(sourceEvent.UserId) {
				bot.SendMessage(sourceEvent, "You don't have permission to do that.")
				return
			}
		}

		query := `DELETE FROM reminders WHERE id=?;`

		if _, err := bot.Db.Exec(query, id); err != nil {
			bot.Log.Warningf("Error while deleting a reminder: %s", err)
			bot.SendMessage(sourceEvent, fmt.Sprintf("Error: %s", err))
			return
		}
		bot.SendMessage(sourceEvent, fmt.Sprintf("Removed reminder number %d.", id))
		// Reload  counters.
		ext.loadReminders()
		return
	}

	// Add.
	if len(params) > 3 && command == "add" {
		// Sanity check parameters.
		delay, err := bot.Humanizer.ParseDuration(params[1] + " " + params[2])
		if err != nil {
			bot.SendMessage(sourceEvent, fmt.Sprintf("%s", err))
			return
		}
		createTime := time.Now()
		targetTime := createTime.Add(delay)

		text := strings.Join(params[3:], " ")
		// Add reminder to database.
		query := `
			INSERT INTO reminders (transport, channel, creator, announce_text, announced, target_time, created_time)
			VALUES (?, ?, ?, ?, ?, ?, ?);
			`
		if _, err := bot.Db.Exec(query, sourceEvent.TransportName, sourceEvent.Channel, sourceEvent.Nick, text,
			0, targetTime.Format("2006-01-02 15:04:05"), createTime.Format("2006-01-02 15:04:05")); err != nil {
			bot.Log.Warningf("Error while adding a reminder: %s", err)
			bot.SendMessage(sourceEvent, fmt.Sprintf("Error: %s", err))
			return
		}
		bot.SendMessage(sourceEvent, fmt.Sprintf("Reminder set for %s.", targetTime.Format("2006-01-02 15:04:05")))
		// Reload reminders.
		ext.loadReminders()
		return
	}
}
