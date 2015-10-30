package papaBot

import (
	"fmt"
	"github.com/mvdan/xurls"
	"golang.org/x/net/html/charset"
	_ "golang.org/x/net/html/charset"
	"golang.org/x/net/idna"
	"golang.org/x/text/transform"
	"html"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

var (
	httpClient               = http.Client{Timeout: 5 * time.Second}
	titleRe                  *regexp.Regexp
	lastAnnouncedTime        = map[string]time.Time{}
	lastAnnouncedLinesPassed = map[string]int{}
)

const (
	UserAgent               = "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)"
	announceIntervalMinutes = 15
	announceIntervalLines   = 50
	preloadBodySize         = 20 * 1024
)

// Initializes stuff needed by this processor.
func initUrlProcessor(bot *Bot) error {
	titleRe, _ = regexp.Compile("<title.*?>(.+?)</title>")

	// Create URLs table if needed
	query := `
		CREATE TABLE IF NOT EXISTS "urls" (
			"id" INTEGER PRIMARY KEY  AUTOINCREMENT  NOT NULL,
			"channel" VARCHAR NOT NULL,
			"nick" VARCHAR NOT NULL,
			"link" VARCHAR NOT NULL,
			"quote" VARCHAR NOT NULL,
			"title" VARCHAR,
			"timestamp" DATETIME DEFAULT (datetime('now','localtime'))
		);`
	if _, err := bot.Db.Exec(query); err != nil {
		return err
	}
	// FTS table
	query = `
		CREATE VIRTUAL TABLE IF NOT EXISTS urls_search
		USING fts4(channel, nick, link, title, timestamp, search);`
	if _, err := bot.Db.Exec(query); err != nil {
		return err
	}
	// FTS trigger
	query = `
		CREATE TRIGGER IF NOT EXISTS url_add AFTER INSERT ON urls BEGIN
			INSERT INTO urls_search(channel, nick, link, title, timestamp, search)
			VALUES(new.channel, new.nick, new.link, new.title, new.timestamp, new.link || ' ' || new.title);
		END`
	if _, err := bot.Db.Exec(query); err != nil {
		return err
	}
	query = `
		CREATE TRIGGER IF NOT EXISTS url_update AFTER UPDATE ON urls BEGIN
			UPDATE urls_search SET title = new.title, search = new.link || ' ' || new.title
			WHERE timestamp = new.timestamp;
		END`
	if _, err := bot.Db.Exec(query); err != nil {
		return err
	}
	return nil
}

// Standardizes the url.
func standardize(url string) string {
	link := url
	var schema, domain, path string

	// Try to get the schema
	slice := strings.SplitN(url, "://", 2)
	if len(slice) == 2 && len(slice[0]) < 10 { // schema exists
		schema = slice[0] + "://"
		link = slice[1]
	} else {
		schema = "http://"
	}

	// Get the domain
	slice = strings.SplitN(link, "/", 2)
	if len(slice) == 2 {
		domain = slice[0]
		path = slice[1]
	} else {
		domain = slice[0]
		path = ""
	}

	domain, _ = idna.ToASCII(domain)
	link = schema + domain + "/" + path

	if !strings.HasSuffix(link, "/") {
		link = link + "/"
	}
	return link
}

// Find the title for url.
func getTitle(bot *Bot, url string) string {
	// Build the request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		bot.logWarn.Println("Can't build request for", url)
		return ""
	}
	req.Header.Set("User-Agent", UserAgent)

	// Get response
	resp, err := httpClient.Do(req)
	if err != nil {
		bot.logWarn.Println("Can't get the response for", url)
		return ""
	}
	defer resp.Body.Close()

	// Load part of body
	body := make([]byte, preloadBodySize, preloadBodySize)
	if _, err := io.ReadFull(resp.Body, body); err != nil {
		bot.logWarn.Println("Can't load body for:", url)
		return ""
	}
	// Get the content-type
	content_type := resp.Header.Get("Content-Type")
	if content_type == "" {
		content_type = http.DetectContentType(body)
	}
	// Detect the encoding and create decoder
	encoding, _, _ := charset.DetermineEncoding(body, content_type)
	if strings.Contains(content_type, "text/html") {
		// get the title
		match := titleRe.FindStringSubmatch(string(body))
		if len(match) > 1 {
			title, _, _ := transform.String(encoding.NewDecoder(), match[1])
			title = html.UnescapeString(title)
			bot.logDebug.Println("Found title:", title)
			return title
		}
	} else {
		bot.logWarn.Println("Not checking title for content type:", content_type)
		return ""
	}

	return ""
}

// Check for duplicates of the url in the database.
func checkForDuplicates(bot *Bot, channel, sender, link string) string {
	result, err := bot.Db.Query(`
		SELECT IFNULL(nick, ""), IFNULL(timestamp, datetime('now')), count(*)
		FROM urls WHERE link=? AND channel=?
		ORDER BY timestamp DESC LIMIT 1`, link, channel)
	if err != nil {
		bot.logWarn.Println("Can't query the database for duplicates:", err)
		return ""
	}
	defer result.Close()

	// Announce a duplicate
	if result.Next() {
		var nick string
		var timestr string
		var count uint
		if err = result.Scan(&nick, &timestr, &count); err != nil {
			bot.logWarn.Println("Error getting duplicates:", err)
			return ""
		}
		timestamp, _ := time.Parse("2006-01-02 15:04:05", timestr)

		// Only one duplicate
		if count == 1 {
			if bot.areSamePeople(nick, sender) {
				nick = bot.Texts.DuplicateYou
			}
			elapsed := GetTimeElapsed(timestamp)
			return Format(bot.Texts.tempDuplicateFirst, &map[string]string{"nick": nick, "elapsed": elapsed})
		} else if count > 1 { // More duplicates exist
			if bot.areSamePeople(nick, sender) {
				nick = bot.Texts.DuplicateYou
			}
			elapsed := GetTimeElapsed(timestamp)
			return Format(
				bot.Texts.tempDuplicateMulti,
				&map[string]string{"nick": nick, "elapsed": elapsed, "count": fmt.Sprintf("%d", count)})
		}
	}
	return ""
}

// Find out what to announce to the channel.
func announce(channel, title, link, duplicates string) string {

	// If we can't announce yet, return
	if time.Since(lastAnnouncedTime[link+channel]) < announceIntervalMinutes*time.Minute {
		return ""
	}
	if lines, exists := lastAnnouncedLinesPassed[link+channel]; exists && lines < announceIntervalLines {
		return ""
	}

	if title != "" && duplicates != "" { // Announce both title and duplicates at the same time.
		lastAnnouncedTime[link+channel] = time.Now()
		lastAnnouncedLinesPassed[link+channel] = 0
		return fmt.Sprintf("%s (%s)", title, duplicates)
	}

	if title != "" {
		lastAnnouncedTime[link+channel] = time.Now()
		lastAnnouncedLinesPassed[link+channel] = 0
		return title
	}

	if duplicates != "" {
		lastAnnouncedTime[link+channel] = time.Now()
		lastAnnouncedLinesPassed[link+channel] = 0
		return duplicates
	}

	return ""
}

// Look for urls in the message, resolve the title.
func processorURLs(bot *Bot, channel, sender, msg string) {
	// Increase lines count for all announcements
	for k := range lastAnnouncedLinesPassed {
		lastAnnouncedLinesPassed[k] += 1
		// After 100 lines pass, forget it ever happened
		if lastAnnouncedLinesPassed[k] > 100 {
			delete(lastAnnouncedLinesPassed, k)
			delete(lastAnnouncedTime, k)
		}
	}

	links := xurls.Relaxed.FindAllString(msg, -1)
	for i := range links {
		// Validate the url
		bot.logInfo.Println("Got link:", links[i])
		link := standardize(links[i])
		bot.logInfo.Println("Standardized to:", link)

		// Get the title
		title := getTitle(bot, link)

		// Check for duplicates
		duplicates := checkForDuplicates(bot, channel, sender, link)

		// What to announce?
		if msg := announce(channel, title, link, duplicates); msg != "" {
			bot.SendNotice(channel, msg)
		}

		// Insert url into db
		_, err := bot.Db.Exec(`
			INSERT INTO urls(channel, nick, link, quote, title) VALUES(?, ?, ?, ?, ?)`,
			channel, sender, link, msg, title)
		if err != nil {
			bot.logWarn.Println("Can't add url to database:", err)
		}
	}
}
