package papaBot

import (
	"github.com/mvdan/xurls"
	"golang.org/x/net/idna"
	"net/http"
	"regexp"
	"strings"
	"time"
	"html"
)

var (
	httpClient http.Client
	titleRe    *regexp.Regexp
	lastAnnouncedTitle map[string]time.Time
	lastAnnouncedDuplicate map[string]time.Time
)

const UserAgent = "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)"

func init() {
	httpClient = http.Client{
		Timeout: 5 * time.Second,
	}
	titleRe, _ = regexp.Compile("<title>(.+?)</title>")
	lastAnnouncedTitle = map[string]time.Time{}
	lastAnnouncedDuplicate = map[string]time.Time{}
}

// Make sure the address has a schema
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
	link = schema + domain + path

	if !strings.HasSuffix(link, "/") {
		link = link + "/"
	}
	return link
}

// Find the title for url
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
	body := make([]byte, 10*1024, 10*1024)
	if _, err = resp.Body.Read(body); err != nil {
		bot.logWarn.Println("Can't load body for:", url)
		return ""
	}

	// Get the content-type
	content_type := resp.Header.Get("Content-Type")
	if content_type == "" {
		content_type = http.DetectContentType(body)
	}
	if strings.Contains(content_type, "text/html") {
		// get the title
		match := titleRe.FindString(string(body))
		if match != "" {
			title := strings.Replace(match, "<title>", "", -1)
			title = strings.Replace(title, "</title>", "", -1)
			title = html.UnescapeString(title)
			bot.logInfo.Println("Found title:", title)
			return title
		}
	} else {
		bot.logWarn.Println("Not checking title for content type:", content_type)
		return ""
	}

	return ""
}

// Check for duplicates of the url in the database
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
			if AreSamePeople(nick, sender) {
				nick = txtDuplicateYou
			}
			elapsed := GetTimeElapsed(timestamp)
			return text(txtDuplicateFirst, nick, elapsed)
		} else if count > 1 { // More duplicates exist
			if AreSamePeople(nick, sender) {
				nick = txtDuplicateYou
			}
			elapsed := GetTimeElapsed(timestamp)
			return text(txtDuplicateMulti, count, nick, elapsed)
		}
	}
	return ""
}

// Look for urls in the message, resolve the title
func processorURLs(bot *Bot, channel, sender, msg string) {
	links := xurls.Relaxed.FindAllString(msg, -1)
	for i := range links {
		// Validate the url
		bot.logInfo.Println("Got link:", links[i])
		link := standardize(links[i])
		bot.logInfo.Println("Standardized to:", link)

		// Get the title
		title := getTitle(bot, link)
		// Announce the title
		if title != "" {
			// Do not announce title too often
			lastTime, exists := lastAnnouncedTitle[title+channel]
			if exists && time.Since(lastTime) < 5 * time.Minute {
				// do nothing
			} else {
				bot.SendNotice(channel, title)
				lastAnnouncedTitle[title + channel] = time.Now()
			}
		}
		// Check for duplicates
		duplicates := checkForDuplicates(bot, channel, sender, link)
		// Announce the duplicates
		if duplicates != "" {
			// Do not announce duplicate of the same link too often
			lastTime, exists := lastAnnouncedDuplicate[link + channel]
			if exists && time.Since(lastTime) < 5 * time.Minute {
				// do nothing
			} else {
				bot.SendNotice(channel, duplicates)
				lastAnnouncedDuplicate[link + channel] = time.Now()
			}
		}

		// Insert url into db
		_, err := bot.Db.Exec("INSERT INTO urls(channel, nick, link, quote, title) VALUES(?, ?, ?, ?, ?)",
			channel, sender, link, msg, title)
		if err != nil {
			bot.logWarn.Println("Can't add url to database:", err)
		}
	}
	bot.logInfo.Println("Finished processing URLs.")
}
