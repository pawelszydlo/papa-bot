package papaBot

import (
	"fmt"
	"github.com/pawelszydlo/papa-bot/utils"
	"math"
	"strconv"
	"strings"
	"text/template"
	"time"
)

// ExtensionCounters - enables the creation of custom counters.
type ExtensionCounters struct {
	counters map[int]*extensionCountersCounter
}

type extensionCountersCounter struct {
	channel  string
	creator  string
	text     string
	textTmp  *template.Template
	interval time.Duration
	date     time.Time
	nextTick time.Time
}

// message will produce an announcement message for the counter.
func (cs *extensionCountersCounter) message() string {
	diff := time.Since(cs.date)
	days := int(math.Abs(diff.Hours())) / 24
	hours := int(math.Abs(diff.Hours())) - days*24
	minutes := int(math.Abs(diff.Minutes())) - hours*60 - days*1440
	vars := map[string]string{
		"days":    fmt.Sprintf("%d", days),
		"hours":   fmt.Sprintf("%d", hours),
		"minutes": fmt.Sprintf("%d", minutes),
		"since":   HumanizedSince(cs.date),
	}
	return utils.Format(cs.textTmp, vars)
}

// Init initializes the extension.
func (ext *ExtensionCounters) Init(bot *Bot) error {
	// Create database table to hold the counters.
	query := `
		-- Main URLs table.
		CREATE TABLE IF NOT EXISTS "counters" (
			"id" INTEGER PRIMARY KEY  AUTOINCREMENT  NOT NULL,
			"channel" VARCHAR NOT NULL,
			"creator" VARCHAR NOT NULL,
			"announce_text" VARCHAR NOT NULL,
			"interval" INTEGER NOT NULL,
			"target_date" VARCHAR NOT NULL,
			"created" DATETIME DEFAULT (datetime('now','localtime')),
			FOREIGN KEY(creator) REFERENCES users(nick)
		);`
	if _, err := bot.Db.Exec(query); err != nil {
		bot.log.Panic(err)
	}

	// Add commands for handling the counters.
	bot.commands["counter"] = &BotCommand{
		true, false, true,
		"counter help | list | announce [id] | del [id] | add [date] [time] [interval] [channel] [text]",
		"Controls custom counters.",
		ext.commandCounters}

	// Load counters from the db.
	ext.loadCounters(bot)

	return nil
}

// Tick will announce all the counters if needed.
func (ext *ExtensionCounters) Tick(bot *Bot, daily bool) {
	// Check if it's time to announce the counter.
	for id, c := range ext.counters {
		if time.Since(c.nextTick) > 0 {
			bot.SendNotice(c.channel, c.message())
			c.nextTick = c.nextTick.Add(c.interval * time.Hour)
			bot.log.Debug("Counter %d, next tick: %s", id, c.nextTick)
		}
	}
}

// loadCounters will load the counters from the database.
func (ext *ExtensionCounters) loadCounters(bot *Bot) {
	ext.counters = map[int]*extensionCountersCounter{}

	result, err := bot.Db.Query(
		`SELECT id, channel, creator, announce_text, interval, target_date FROM counters`)
	if err != nil {
		bot.log.Warning("Error while loading counters: %s", err)
		return
	}
	defer result.Close()

	// Get vars.
	for result.Next() {
		var c extensionCountersCounter
		var date string
		var id int
		var interval int
		if err = result.Scan(&id, &c.channel, &c.creator, &c.text, &interval, &date); err != nil {
			bot.log.Warning("Can't load counter: %s", err)
			continue
		}
		c.interval = time.Duration(interval)
		// Parse the text template.
		c.textTmp, err = template.New(fmt.Sprintf("counter_%d", id)).Parse(c.text)
		if err != nil {
			bot.log.Warning("Can't parse counter template '%s': %s", c.text, err)
		}
		// Handle the date.
		c.date, _ = time.Parse("2006-01-02 15:04:05", date)
		c.date = utils.MustForceLocalTimezone(c.date)
		// Calculate next tick. Start from next daily tick and move backwards.
		nextTick := bot.nextDailyTick
		for {
			c.nextTick = nextTick
			nextTick = nextTick.Add(-time.Duration(c.interval) * time.Hour)
			if time.Since(nextTick) > 0 { // We moved too far back.
				break
			}
		}
		bot.log.Debug("Counter %d, next tick: %s", id, c.nextTick)

		ext.counters[id] = &c
	}
}

// commandCounters is a command for handling the counters.
func (ext *ExtensionCounters) commandCounters(
	bot *Bot, nick, user, channel, receiver string, priv bool, params []string) {

	if len(params) < 1 {
		return
	}
	command := params[0]
	fullName := nick + "!" + user

	// List.
	if command == "list" {
		if len(ext.counters) > 0 {
			bot.SendMessage(receiver, "Counters:")
			for id, c := range ext.counters {
				bot.SendMessage(receiver, fmt.Sprintf(
					"%d: %s | %s | interval %dh | %s", id, c.channel, c.date, c.interval, c.text))
			}
		} else {
			bot.SendMessage(receiver, "No counters yet.")
		}
		return
	}

	if command == "help" {
		bot.SendMessage(nick, "To add a new counter:")
		bot.SendMessage(nick, "add [date] [time] [interval] [channel] [text]")
		bot.SendMessage(nick, `Where: date in format 'YYYY-MM-DD', time in format 'HH:MM:SS', interval is annouce
			interval in hours, channel is the name of the channel to announce on, text is the announcement text.`)
		bot.SendMessage(
			nick, "Announcement text may contain placeholders: {{ .days }}, {{ .hours }}, {{ .minutes }}, {{ .since }}")
		return
	}

	// Force announce.
	if len(params) == 2 && command == "announce" {
		id, err := strconv.Atoi(params[1])
		if err != nil || ext.counters[id] == nil {
			bot.SendMessage(receiver, "Wrong id.")
			return
		}
		bot.SendMessage(receiver, fmt.Sprintf("Announcing counter %d to %s...", id, ext.counters[id].channel))
		bot.SendMessage(ext.counters[id].channel, ext.counters[id].message())
	}

	// Delete.
	if len(params) == 2 && command == "del" {
		id := params[1]
		bot.SendMessage(receiver, fmt.Sprintf("Deleting counter number %s...", id))
		query := ""
		// Bot owner can delete all counters.
		if bot.userIsOwner(fullName) {
			query = `DELETE FROM counters WHERE id=?;`
		} else {
			// User must be an admin, he can delete only his own counters.
			nick := bot.authenticatedAdmins[fullName]
			query = fmt.Sprintf(`DELETE FROM counters WHERE id=? AND creator="%s";`, nick)
		}
		if _, err := bot.Db.Exec(query, id); err != nil {
			bot.log.Warning("Error while deleting a counter: %s", err)
			bot.SendMessage(receiver, fmt.Sprintf("Error: %s", err))
			return
		}
		// Reload  counters.
		ext.loadCounters(bot)
		return
	}

	// Add.
	if len(params) > 5 && command == "add" {
		// Sanity check parameters.
		date, err := time.Parse("2006-01-0215:04:05", params[1]+params[2])
		if err != nil {
			bot.SendMessage(receiver, "Date and time must be in format: 2015-12-31 12:54:00")
			return
		}
		interval, err := strconv.ParseInt(params[3], 10, 32)
		if err != nil {
			bot.SendMessage(receiver, "interval parameter must be a number.")
			return
		}
		channel = params[4]
		if !strings.HasPrefix(channel, "#") {
			bot.SendMessage(receiver, "invalid channel name: "+channel)
			return
		}

		text := strings.Join(params[5:], " ")
		nick := bot.authenticatedAdmins[fullName]
		// Add counter to database.
		query := `
			INSERT INTO counters (channel, creator, announce_text, interval, target_date)
			VALUES (?, ?, ?, ?, ?);
			`
		if _, err := bot.Db.Exec(query, channel, nick, text, interval, date); err != nil {
			bot.log.Warning("Error while adding a counter: %s", err)
			bot.SendMessage(receiver, fmt.Sprintf("Error: %s", err))
			return
		}
		bot.SendMessage(receiver, "Counter created.")
		// Reload  counters.
		ext.loadCounters(bot)
		return
	}
}

// Not implemented.
func (ext *ExtensionCounters) ProcessURL(bot *Bot, urlinfo *UrlInfo, channel, sender, msg string) {}
func (ext *ExtensionCounters) ProcessMessage(bot *Bot, channel, sender, msg string)               {}
