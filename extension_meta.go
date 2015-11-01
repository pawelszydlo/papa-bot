package papaBot

import (
	"fmt"
	"regexp"
	"text/template"
	"time"
)

type ExtensionMeta struct {
	Texts                   *ExtensionMetaTexts
	titleRe, metaRe, descRe *regexp.Regexp
}

type ExtensionMetaTexts struct {
	TempDuplicateFirst *template.Template
	TplDuplicateFirst  string

	TempDuplicateMulti *template.Template
	TplDuplicateMulti  string

	DuplicateYou string
}

// Init the extension.
func (proc *ExtensionMeta) Init(bot *Bot) error {
	texts := new(ExtensionMetaTexts) // Can't load directly because of reflection issues.
	if err := bot.loadTexts(bot.textsFile, texts); err != nil {
		return err
	}
	proc.Texts = texts
	proc.titleRe = regexp.MustCompile("(?is)<title.*?>(.+?)</title>")
	proc.metaRe = regexp.MustCompile(`(?is)<\s*?meta.*?content\s*?=\s*?"(.*?)".*?>`)
	proc.descRe = regexp.MustCompile(`(?is)(property|name)\s*?=.*?description`)
	return nil
}

// Find the title for url.
func (proc *ExtensionMeta) getTitle(body string) (string, string, error) {
	// Iterate over meta tags to get the description
	description := ""
	metas := proc.metaRe.FindAllStringSubmatch(string(body), -1)
	for i := range metas {
		if len(metas[i]) > 1 {
			isDesc := proc.descRe.FindString(metas[i][0])
			if isDesc != "" && (len(metas[i][1]) > len(description)) {
				description = SanitizeString(metas[i][1])
			}
		}
	}
	// Get the title
	match := proc.titleRe.FindStringSubmatch(string(body))
	if len(match) > 1 {
		title := SanitizeString(match[1])
		return title, description, nil
	}

	return "", "", nil
}

// Check for duplicates of the url in the database.
func (proc *ExtensionMeta) checkForDuplicates(bot *Bot, channel, sender, link string) string {
	result, err := bot.Db.Query(`
		SELECT IFNULL(nick, ""), IFNULL(timestamp, datetime('now')), count(*)
		FROM urls WHERE link=? AND channel=?
		ORDER BY timestamp DESC LIMIT 1`, link, channel)
	if err != nil {
		bot.log.Warning("Can't query the database for duplicates: %s", err)
		return ""
	}
	defer result.Close()

	// Announce a duplicate
	if result.Next() {
		var nick string
		var timestr string
		var count uint
		if err = result.Scan(&nick, &timestr, &count); err != nil {
			bot.log.Warning("Error getting duplicates: %s", err)
			return ""
		}
		timestamp, _ := time.Parse("2006-01-02 15:04:05", timestr)

		// Only one duplicate
		if count == 1 {
			if bot.areSamePeople(nick, sender) {
				nick = proc.Texts.DuplicateYou
			}
			elapsed := GetTimeElapsed(timestamp)
			return Format(proc.Texts.TempDuplicateFirst, &map[string]string{"nick": nick, "elapsed": elapsed})
		} else if count > 1 { // More duplicates exist
			if bot.areSamePeople(nick, sender) {
				nick = proc.Texts.DuplicateYou
			}
			elapsed := GetTimeElapsed(timestamp)
			return Format(
				proc.Texts.TempDuplicateMulti,
				&map[string]string{"nick": nick, "elapsed": elapsed, "count": fmt.Sprintf("%d", count)})
		}
	}
	return ""
}

// Find out what to announce to the channel.
func (proc *ExtensionMeta) prepareShort(title, duplicates string) string {
	if title != "" && duplicates != "" {
		return fmt.Sprintf("%s (%s)", title, duplicates)
	}
	if title != "" {
		return title
	}
	if duplicates != "" {
		return duplicates
	}
	return ""
}

// Look for urls in the message, resolve the title.
func (proc *ExtensionMeta) ProcessURL(bot *Bot, urlinfo *UrlInfo, channel, sender, msg string) {
	if len(urlinfo.Body) == 0 {
		return
	}
	// Get the title
	title, description, err := proc.getTitle(string(urlinfo.Body))
	if err != nil {
		bot.log.Warning("Error getting title: %s", err)
	} else {
		bot.log.Debug("Title: %s", title)
	}

	// Check for duplicates
	duplicates := proc.checkForDuplicates(bot, channel, sender, urlinfo.URL)
	urlinfo.ShortInfo = proc.prepareShort(title, duplicates)
	urlinfo.LongInfo = description

	// Insert url into db
	_, err = bot.Db.Exec(`
		INSERT INTO urls(channel, nick, link, quote, title) VALUES(?, ?, ?, ?, ?)`,
		channel, sender, urlinfo.URL, msg, title)
	if err != nil {
		bot.log.Warning("Can't add url to database: %s", err)
	}
}
