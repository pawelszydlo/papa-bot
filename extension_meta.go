package papaBot

import (
	"fmt"
	"regexp"
	"strings"
	"text/template"
	"time"
)

// ExtensionMeta - extension for getting title and description from html links.
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

// Init inits the extension.
func (ext *ExtensionMeta) Init(bot *Bot) error {
	texts := new(ExtensionMetaTexts) // Can't load directly because of reflection issues.
	if err := bot.loadTexts(bot.textsFile, texts); err != nil {
		return err
	}
	ext.Texts = texts
	ext.titleRe = regexp.MustCompile("(?is)<title.*?>(.+?)</title>")
	ext.metaRe = regexp.MustCompile(`(?is)<\s*?meta.*?content\s*?=\s*?"(.*?)".*?>`)
	ext.descRe = regexp.MustCompile(`(?is)(property|name)\s*?=.*?description`)
	return nil
}

// getTitle find the title and description.
func (ext *ExtensionMeta) getTitle(body string) (string, string, error) {
	// Iterate over meta tags to get the description
	description := ""
	metas := ext.metaRe.FindAllStringSubmatch(string(body), -1)
	for i := range metas {
		if len(metas[i]) > 1 {
			isDesc := ext.descRe.FindString(metas[i][0])
			if isDesc != "" && (len(metas[i][1]) > len(description)) {
				description = CleanString(metas[i][1])
			}
		}
	}
	// Get the title
	match := ext.titleRe.FindStringSubmatch(string(body))
	if len(match) > 1 {
		title := CleanString(match[1])
		return title, description, nil
	}

	return "", "", nil
}

// checkForDuplicates checks for duplicates of the url in the database.
func (ext *ExtensionMeta) checkForDuplicates(bot *Bot, channel, sender, link string) string {
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
				nick = ext.Texts.DuplicateYou
			}
			elapsed := HumanizedSince(MustForceLocalTimezone(timestamp))
			return Format(ext.Texts.TempDuplicateFirst, &map[string]string{"nick": nick, "elapsed": elapsed})
		} else if count > 1 { // More duplicates exist
			if bot.areSamePeople(nick, sender) {
				nick = ext.Texts.DuplicateYou
			}
			elapsed := HumanizedSince(MustForceLocalTimezone(timestamp))
			return Format(
				ext.Texts.TempDuplicateMulti,
				&map[string]string{"nick": nick, "elapsed": elapsed, "count": fmt.Sprintf("%d", count)})
		}
	}
	return ""
}

// prepareShort finds out what to announce to the channel.
func (ext *ExtensionMeta) prepareShort(title, duplicates string) string {
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

// ProcessURL will try to get the title and description.
func (ext *ExtensionMeta) ProcessURL(bot *Bot, urlinfo *UrlInfo, channel, sender, msg string) {
	if len(urlinfo.Body) == 0 || !strings.Contains(urlinfo.ContentType, "html") {
		return
	}
	// Get the title
	title, description, err := ext.getTitle(string(urlinfo.Body))
	if err != nil {
		bot.log.Warning("Error getting title: %s", err)
	} else {
		bot.log.Debug("Title: %s", title)
	}

	// Check for duplicates
	duplicates := ext.checkForDuplicates(bot, channel, sender, urlinfo.URL)
	urlinfo.ShortInfo = ext.prepareShort(title, duplicates)
	urlinfo.LongInfo = description

	// Insert url into db
	_, err = bot.Db.Exec(`
		INSERT INTO urls(channel, nick, link, quote, title) VALUES(?, ?, ?, ?, ?)`,
		channel, sender, urlinfo.URL, msg, title)
	if err != nil {
		bot.log.Warning("Can't add url to database: %s", err)
	}
}
