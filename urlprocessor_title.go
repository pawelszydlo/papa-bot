package papaBot

import (
	"errors"
	"fmt"
	"golang.org/x/net/html/charset"
	_ "golang.org/x/net/html/charset"
	"io"
	"net/http"
	"regexp"
	"strings"
	"text/template"
	"time"
)

type urlProcessorTitleTextsStruct struct {
	TempDuplicateFirst *template.Template
	TplDuplicateFirst  string

	TempDuplicateMulti *template.Template
	TplDuplicateMulti  string

	DuplicateYou string
}

var (
	titleRe    = regexp.MustCompile("(?is)<title.*?>(.+?)</title>")
	metaRe     = regexp.MustCompile(`(?is)<\s*?meta.*?content\s*?=\s*?"(.*?)".*?>`)
	descRe     = regexp.MustCompile(`(?is)(property|name)\s*?=.*?description`)
	titleTexts = &urlProcessorTitleTextsStruct{}
)

// Init the processor.
func initUrlProcessorTitle(bot *Bot) {
	bot.loadTexts(bot.textsFile, titleTexts)
}

// Find the title for url.
func getTitle(url string) (string, string, string, error) {
	// Get response
	resp, err := GetHttpResponse(url)
	if err != nil {
		return "", "", "", err
	}
	defer resp.Body.Close()

	final_link := resp.Request.URL.String()

	// Load part of body
	body := make([]byte, preloadBodySize, preloadBodySize)
	if _, err := io.ReadFull(resp.Body, body); err == io.ErrUnexpectedEOF {
		// No worries, content ended before we filled the buffer
	} else if err != nil {
		return "", final_link, "", err
	}
	// Get the content-type
	content_type := resp.Header.Get("Content-Type")
	if content_type == "" {
		content_type = http.DetectContentType(body)
	}
	// Detect the encoding and create decoder
	encoding, _, _ := charset.DetermineEncoding(body, content_type)
	if strings.Contains(content_type, "text/html") {
		// Iterate over meta tags to get the description
		description := ""
		metas := metaRe.FindAllStringSubmatch(string(body), -1)
		for i := range metas {
			if len(metas[i]) > 1 {
				isDesc := descRe.FindString(metas[i][0])
				if isDesc != "" && (len(metas[i][1]) > len(description)) {
					description = SanitizeString(metas[i][1], encoding)
				}
			}
		}
		// Get the title
		match := titleRe.FindStringSubmatch(string(body))
		if len(match) > 1 {
			title := SanitizeString(match[1], encoding)
			return title, final_link, description, nil
		}
	} else {
		return "", "", "", errors.New("Not checking title for content type: " + content_type)
	}

	return "", final_link, "", nil
}

// Check for duplicates of the url in the database.
func checkForDuplicates(bot *Bot, channel, sender, link string) string {
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
				nick = titleTexts.DuplicateYou
			}
			elapsed := GetTimeElapsed(timestamp)
			return Format(titleTexts.TempDuplicateFirst, &map[string]string{"nick": nick, "elapsed": elapsed})
		} else if count > 1 { // More duplicates exist
			if bot.areSamePeople(nick, sender) {
				nick = titleTexts.DuplicateYou
			}
			elapsed := GetTimeElapsed(timestamp)
			return Format(
				titleTexts.TempDuplicateMulti,
				&map[string]string{"nick": nick, "elapsed": elapsed, "count": fmt.Sprintf("%d", count)})
		}
	}
	return ""
}

// Find out what to announce to the channel.
func prepareShort(title, duplicates string) string {
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
func urlProcessorTitle(bot *Bot, info *urlInfo, channel, sender, msg string) {
	// Get the title
	title, final_link, description, err := getTitle(info.link)
	if err != nil {
		bot.log.Warning("Error getting title: %s", err)
	} else {
		bot.log.Debug("Title: %s", title)
	}
	if final_link != "" && final_link != info.link {
		bot.log.Debug("%s after redirects becomes: %s ", info.link, final_link)
		info.link = final_link
	}

	// Check for duplicates
	duplicates := checkForDuplicates(bot, channel, sender, info.link)
	info.shortInfo = prepareShort(title, duplicates)
	info.longInfo = description

	// Insert url into db
	_, err = bot.Db.Exec(`
		INSERT INTO urls(channel, nick, link, quote, title) VALUES(?, ?, ?, ?, ?)`,
		channel, sender, info.link, msg, title)
	if err != nil {
		bot.log.Warning("Can't add url to database: %s", err)
	}
}
