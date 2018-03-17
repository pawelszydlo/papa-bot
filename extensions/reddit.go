package extensions

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/pawelszydlo/papa-bot"
	"github.com/pawelszydlo/papa-bot/events"
	"github.com/pawelszydlo/papa-bot/utils"
	"math/rand"
	"strings"
	"text/template"
	"time"
)

/*
ExtensionReddit - extension for getting link information from reddit.com.

This extension will automatically try to fetch Reddit information for each URL posted. It will also post a random
link from one of the hot reddits once per day.

Used custom variables:
- interestingReddits - space separated list of subreddits to use for hot links.
- redditDaily - if set, show daily interesting link.
*/
type ExtensionReddit struct {
	announced     map[string]bool
	announcedLive map[string]bool
	Texts         *extensionRedditTexts
	bot           *papaBot.Bot
}

type extensionRedditTexts struct {
	TempRedditAnnounce *template.Template
	TempRedditDaily    *template.Template
	TempRedditBreaking *template.Template
}

// Reddit structs.
type redditPostData struct {
	Id        string
	Subreddit string
	Author    string
	Domain    string
	Score     int
	Comments  int `json:"num_comments"`
	Title     string
	Url       string
	Created_utc   float64
	Stickied  bool
}
type redditListing struct {
	Error int
	Kind  string
	Data  struct {
		Title    string
		Id       string
		Children []struct{ Data redditPostData }
	}
}

func (postData *redditPostData) toStrings(ext *ExtensionReddit) map[string]string {
	// Try to shorten the url if it is a self post.
	if strings.HasPrefix(postData.Domain, "self.") {
		postData.Url = "http://redd.it/" + postData.Id
	}
	return map[string]string{
		"id":           postData.Id,
		"created":      ext.bot.Humanizer.TimeDiffNow(time.Unix(int64(postData.Created_utc), 0), false),
		"author":       postData.Author,
		"subreddit":    postData.Subreddit,
		"score":        ext.bot.Humanizer.PrefixFast(float64(postData.Score)),
		"comments_url": "http://redd.it/" + postData.Id,
		"comments":     fmt.Sprintf("%d", postData.Comments),
		"title":        postData.Title,
		"url":          postData.Url,
	}
}

// getRedditListing fetches a reddit listing data.
func (ext *ExtensionReddit) getRedditListing(url string, listing *redditListing) error {
	// Reddit API doesn't like it when you pretend to be someone else.
	headers := map[string]string{"User-Agent": "PapaBot version " + papaBot.Version}
	err, _, body := ext.bot.GetPageBody(url, headers)
	if err != nil {
		return err
	}

	// Decode JSON.
	if err := json.Unmarshal(body, &listing); err != nil {
		return err
	}

	// Check for reddit error.
	if listing.Error != 0 {
		return errors.New(fmt.Sprintf("Reddit returned error %d.", listing.Error))
	}
	return nil
}

// getRedditInfo fetches information about a link from Reddit.
func (ext *ExtensionReddit) getRedditInfo(url, channel string, format events.Formatting) string {
	// Get the listing.
	url = fmt.Sprintf("https://www.reddit.com/api/info.json?url=%s", url)
	var listing redditListing
	if err := ext.getRedditListing(url, &listing); err != nil {
		ext.bot.Log.Debugf("Error getting reddit's response %d.", listing.Error)
		return ""
	}

	// Find highest rated post and return it.
	message := ""
	bestScore := 0
	for i := range listing.Data.Children {
		postData := listing.Data.Children[i].Data
		if postData.Score > bestScore {
			// Trim the title.
			if len(postData.Title) > 200 {
				postData.Title = postData.Title[:200] + "(…)"
			}
			message = utils.Format(ext.Texts.TempRedditAnnounce, postData.toStrings(ext))
			bestScore = postData.Score
		}
	}
	ext.bot.Log.Debugf("Reddit: %s", message)
	return message
}

// getRedditHot will get a random article from interesting reddits list.
func (ext *ExtensionReddit) getRedditHot() *redditPostData {
	reddits := strings.Split(ext.bot.GetVar("interestingReddits"), " ")
	if len(reddits) == 0 {
		return nil
	}

	subreddit := reddits[rand.Intn(len(reddits))]
	// Get the listing.
	url := fmt.Sprintf("https://www.reddit.com/r/%s/hot.json?limit=3", subreddit)
	var listing redditListing
	if err := ext.getRedditListing(url, &listing); err != nil {
		ext.bot.Log.Debugf("Error getting reddit's response %d.", listing.Error)
		return nil
	}
	// Get random from the 3 hottest articles.
	if len(listing.Data.Children) > 0 {
		post := listing.Data.Children[rand.Intn(len(listing.Data.Children))].Data
		return &post
	}
	return nil
}

// getRedditLiveNow will get a live now link from the top page, if such exists.
func (ext *ExtensionReddit) getRedditLiveNow() (string, string) {
	// Get the listing.
	url := "https://www.reddit.com/api/live/happening_now.json"
	var liveNow redditListing
	if err := ext.getRedditListing(url, &liveNow); err != nil {
		ext.bot.Log.Debugf("Error getting reddit's response %d.", liveNow.Error)
		return "", ""
	}
	return fmt.Sprintf("https://reddit.com/live/%s", liveNow.Data.Id), liveNow.Data.Title
}

// commandReddit will display one of the interesting articles from Reddit.
func (ext *ExtensionReddit) commandReddit(bot *papaBot.Bot, sourceEvent *events.EventMessage, params []string) {
	post := ext.getRedditHot()
	if post != nil {
		data := post.toStrings(ext)
		message := ""
		if sourceEvent.TransportFormatting == events.FormatMarkdown {
			message = fmt.Sprintf("%s, [%s (/r/%s)](%s)", sourceEvent.Nick, data["title"], data["subreddit"], data["url"])
		} else {
			message = fmt.Sprintf("%s, %s (/r/%s - %s)", sourceEvent.Nick, data["title"], data["subreddit"], data["url"])
		}
		bot.SendMessage(sourceEvent, message)
	}
}

// Init inits the extension.
func (ext *ExtensionReddit) Init(bot *papaBot.Bot) error {
	// Check if user has set any interesting reddits.
	if reddits := bot.GetVar("interestingReddits"); reddits == "" {
		bot.Log.Warningf("No interesting Reddits set in the 'interestingReddits' variable. Setting default.")
		bot.SetVar("interestingReddits",
			"TrueReddit TrueTrueReddit foodforthought Futurology longtext worldnews DepthHub")
	}

	bot.Log.Debugf("Interesting reddits set: %s", bot.GetVar("interestingReddits"))

	// Add command for getting an interesting article.
	bot.RegisterCommand(&papaBot.BotCommand{
		[]string{"reddit", "r"},
		false, false, false,
		"", "Will try to find something interesting to read from Reddit.",
		ext.commandReddit})

	// Load texts.
	ext.announced = map[string]bool{}
	ext.announcedLive = map[string]bool{}
	texts := &extensionRedditTexts{}
	if err := bot.LoadTexts("reddit", texts); err != nil {
		return err
	}
	ext.Texts = texts
	ext.bot = bot
	bot.EventDispatcher.RegisterListener(events.EventTick, ext.TickListener)
	bot.EventDispatcher.RegisterListener(events.EventDailyTick, ext.DailyTickListener)
	bot.EventDispatcher.RegisterListener(events.EventURLFound, ext.ProcessURLListener)
	return nil
}

// DailyTickListener will clear the announces table and give post of the day.
func (ext *ExtensionReddit) DailyTickListener(message events.EventMessage) {
	// Clear the announced list.
	ext.announced = map[string]bool{}
	if ext.bot.GetVar("redditDaily") != "" {
		post := ext.getRedditHot()
		if post != nil {
			ext.bot.SendMassNotice(utils.Format(ext.Texts.TempRedditDaily, post.toStrings(ext)))
		}
	}
}

// TickListener will check for reddit live pinned topics.
func (ext *ExtensionReddit) TickListener(message events.EventMessage) {

	url, title := ext.getRedditLiveNow()
	if url == "" || title == "" {
		return
	}
	if ext.announcedLive[url] {
		return
	}
	ext.announcedLive[url] = true
	ext.bot.SendMassNotice(utils.Format(ext.Texts.TempRedditBreaking, map[string]string{"url": url, "title": title}))

}

// ProcessURLListener will try to check if link was ever on reddit.
func (ext *ExtensionReddit) ProcessURLListener(message events.EventMessage) {
	// Announce each link only once.
	if ext.announced[message.Channel+message.Message] {
		ext.bot.Log.Debugf("Not looking up on reddit, too soon.")
		return
	}
	ext.announced[message.Channel+message.Message] = true
	// Send a notice with URL info.
	go func() {
		reddit := ext.getRedditInfo(message.Message, message.Channel, message.TransportFormatting)
		if reddit != "" {
			ext.bot.SendNotice(&message, reddit)
		}
	}()

}
