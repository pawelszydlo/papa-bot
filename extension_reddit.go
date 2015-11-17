package papaBot

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/pawelszydlo/papa-bot/utils"
	"math/rand"
	"text/template"
	"time"
)

// ExtensionReddit - extension for getting link information from reddit.com.
type extensionReddit struct {
	Extension
	announced          map[string]bool
	Texts              *extensionRedditTexts
	InterestingReddits []string
}

type extensionRedditTexts struct {
	TplRedditAnnounce  string
	TempRedditAnnounce *template.Template
	TplRedditDaily     string
	TempRedditDaily    *template.Template
}

// Reddit structs.
type redditPostData struct {
	Id        string
	Subreddit string
	Author    string
	Score     int
	Comments  int `json:"num_comments"`
	Title     string
	Url       string
	Created   float64
}
type redditListing struct {
	Error int
	Data  struct {
		Children []struct{ Data redditPostData }
	}
}

func (postData *redditPostData) toStrings() map[string]string {
	return map[string]string{
		"id":           postData.Id,
		"created":      utils.HumanizedSince(time.Unix(int64(postData.Created), 0)),
		"author":       postData.Author,
		"subreddit":    postData.Subreddit,
		"score":        fmt.Sprintf("%d", postData.Score),
		"comments_url": "http://redd.it/" + postData.Id,
		"comments":     fmt.Sprintf("%d", postData.Comments),
		"title":        postData.Title,
		"url":          postData.Url,
	}
}

// getRedditListing fetches a reddit listing data.
func (ext *extensionReddit) getRedditListing(bot *Bot, url string, listing *redditListing) error {
	// Get response
	var urlinfo UrlInfo
	urlinfo.URL = url
	// Reddit API doesn't like it when you pretend to be someone else.
	headers := map[string]string{"User-Agent": "PapaBot version " + Version}
	if err := bot.GetPageBody(&urlinfo, headers); err != nil {
		return err
	}

	// Decode JSON.
	if err := json.Unmarshal(urlinfo.Body, &listing); err != nil {
		return err
	}

	// Check for reddit error.
	if listing.Error != 0 {
		return errors.New(fmt.Sprintf("Reddit returned error %d.", listing.Error))
	}
	return nil
}

// getRedditInfo fetches information about a link from Reddit.
func (ext *extensionReddit) getRedditInfo(bot *Bot, url, urlTitle, channel string) string {
	// Catch errors.
	defer func() {
		if Debug {
			return
		} // When in debug mode fail on all errors.
		if r := recover(); r != nil {
			bot.log.Error("FATAL ERROR in reddit extension: %s", r)
		}
	}()

	// Get the listing.
	url = fmt.Sprintf("https://www.reddit.com/api/info.json?url=%s", url)
	var listing redditListing
	if err := ext.getRedditListing(bot, url, &listing); err != nil {
		bot.log.Debug("Error getting reddit's response %d.", listing.Error)
		return ""
	}

	ext.announced[channel+url] = true

	// Find highest rated post and return it.
	message := ""
	bestScore := 0
	for i := range listing.Data.Children {
		postData := listing.Data.Children[i].Data
		if postData.Score > bestScore {
			// Was the title already included in the URL title?
			if postData.Title == urlTitle {
				postData.Title = ""
			}
			// Trim the title.
			if len(postData.Title) > 100 {
				postData.Title = postData.Title[:100] + "…"
			}
			message = utils.Format(ext.Texts.TempRedditAnnounce, postData.toStrings())
			bestScore = postData.Score
		}
	}
	bot.log.Debug("Reddit: %s", message)
	return message
}

// Init inits the extension.
func (ext *extensionReddit) Init(bot *Bot) error {
	ext.InterestingReddits = []string{
		"TrueReddit",
		"foodforthought",
		"Futurology",
		"longtext",
	}
	// Load texts.
	ext.announced = map[string]bool{}
	texts := &extensionRedditTexts{}
	if err := bot.LoadTexts(bot.textsFile, texts); err != nil {
		return err
	}
	ext.Texts = texts
	return nil
}

// Tick will clear the announces table and give post of the day.
func (ext *extensionReddit) Tick(bot *Bot, daily bool) {
	if !daily {
		return
	}
	ext.announced = map[string]bool{}

	subreddit := ext.InterestingReddits[rand.Intn(len(ext.InterestingReddits))]
	// Get the listing.
	url := fmt.Sprintf("https://www.reddit.com/r/%s/hot.json?limit=1", subreddit)
	var listing redditListing
	if err := ext.getRedditListing(bot, url, &listing); err != nil {
		bot.log.Debug("Error getting reddit's response %d.", listing.Error)
		return
	}
	// Get the article.
	if len(listing.Data.Children) > 0 {
		post := listing.Data.Children[0].Data
		bot.SendMassNotice(utils.Format(ext.Texts.TempRedditDaily, post.toStrings()))
	}
}

// ProcessURL will try to check if link was ever on reddit.
func (ext *extensionReddit) ProcessURL(bot *Bot, urlinfo *UrlInfo, channel, sender, msg string) {
	// Announce each link only once.
	if ext.announced[channel+urlinfo.URL] {
		return
	}

	// Can we fit into the ShortInfo?
	if urlinfo.ShortInfo == "" {
		urlinfo.ShortInfo = ext.getRedditInfo(bot, urlinfo.URL, urlinfo.Title, channel)
	} else if len(urlinfo.ShortInfo) < 50 {
		reddit := ext.getRedditInfo(bot, urlinfo.URL, urlinfo.Title, channel)
		if reddit != "" {
			urlinfo.ShortInfo += " | " + reddit
		}
	} else { // Better send as separate notcie.
		go func() {
			reddit := ext.getRedditInfo(bot, urlinfo.URL, urlinfo.Title, channel)
			if reddit != "" {
				bot.SendNotice(channel, reddit)
			}
		}()
	}
}
