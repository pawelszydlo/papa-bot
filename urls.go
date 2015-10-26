package main

import (
	"github.com/mvdan/xurls"
	"net/url"
	"net/http"
	"time"
	"strings"
	"regexp"
	"fmt"
)

var (
	httpClient	http.Client
	titleRe		*regexp.Regexp
)

func init() {
	httpClient = http.Client{
		Timeout: 5 * time.Second,
	}
	titleRe, _ = regexp.Compile("<title>(.+?)</title>")
}

// Make sure the address has a schema
func Standardize(url *url.URL) string {
	if url.Scheme == "" {
		url.Scheme = "http"
	}
	link := url.String()
	if !strings.HasSuffix(link, "/") {
		link = link + "/"
	}
	return link
}

// Find the title for url
func GetTitle(url string) string {
	// Get the response
	resp, err := http.Get(url)
	if err != nil {
		lerror.Println("Can't get the response for", url)
		return ""
	}
	defer resp.Body.Close()

	// Load part of body
	body := make([]byte, 10 * 1024, 10 * 1024)
	if _, err = resp.Body.Read(body); err != nil {
		lerror.Println("Can't load body for:", url)
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
			linfo.Println("Found title:", title)
			return title
		}
	} else {
		linfo.Println("Not checking title for content type:", content_type)
		return ""
	}

	return ""
}

// Check for duplicates of the url in the database
func CheckForDuplicates(channel, sender, link string) {
	result, err := db.Query(`
		SELECT nick, timestamp, count(*)
		FROM urls WHERE link=? AND channel=?
		ORDER BY timestamp DESC LIMIT 1`, link, channel)
	if err != nil {
		lerror.Println("Can't query the database for duplicates:", err)
		return
	}
	defer result.Close()

	// Announce a duplicate
	if result.Next() {
		var nick string
		var timestamp time.Time
		var count uint
		if err = result.Scan(&nick, &timestamp, &count); err != nil {
			lerror.Println("Error getting duplicates:", err)
			return
		}
		// Only one duplicate
		if count == 1 {
			if AreSamePeople(nick, sender) {
				nick = txtDuplicateYou
			}
			elapsed := GetTimeElapsed(timestamp)
			IRC.Notice(channel, fmt.Sprintf(txtDuplicateFirst, nick, elapsed))
		} else if count > 1 {  // More duplicates exist
			if AreSamePeople(nick, sender) {
				nick = txtDuplicateYou
			}
			elapsed := GetTimeElapsed(timestamp)
			IRC.Notice(channel, fmt.Sprintf(txtDuplicateMulti, count, nick, elapsed))
		}
	}
}


// Look for urls in the message, resolve the title
func HandleURLs(channel, sender, msg string) {
	links := xurls.Relaxed.FindAllString(msg, -1)
	for i := range links {
		// Validate the url
		linfo.Println("Got link:", links[i])
		link_object, err := url.Parse(links[i])
		if err != nil {
			lerror.Println("Error parsing url", links[i])
			continue
		}
		link := Standardize(link_object)
		title := GetTitle(link)
		// Announce the title
		if title != "" {
			IRC.Notice(channel, title)
		}
		// Check for duplicates
		CheckForDuplicates(channel, sender, link)

		// Insert url into db
		_, err = db.Exec("INSERT INTO urls(channel, nick, link, quote, title) VALUES(?, ?, ?, ?, ?)",
			channel, sender, link, msg, title)
		if err != nil {
			lerror.Println("Can't add url to database:", err)
		}
	}
}
