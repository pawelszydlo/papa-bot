package papaBot

import (
	"encoding/json"
	"fmt"
	"text/template"
	"time"
)

// ExtensionReddit - extension for getting link information from reddit.com.
type ExtensionReddit struct {
	announced map[string]bool
	Texts     *ExtensionRedditTexts
}

type ExtensionRedditTexts struct {
	TplRedditAnnounce  string
	TempRedditAnnounce *template.Template
}

// Init inits the extension.
func (ext *ExtensionReddit) Init(bot *Bot) error {
	ext.announced = map[string]bool{}
	texts := &ExtensionRedditTexts{}
	if err := bot.loadTexts(bot.textsFile, texts); err != nil {
		return err
	}
	ext.Texts = texts
	return nil
}

func (ext *ExtensionReddit) getRedditInfo(bot *Bot, url, channel string) string {
	// Catch errors.
	defer func() {
		if r := recover(); r != nil {
			bot.log.Error("FATAL ERROR in reddit extension: %s", r)
		}
	}()
	// Get response
	body, err := bot.GetPageBodyByURL(fmt.Sprintf("https://www.reddit.com/api/info.json?url=%s", url))
	if err != nil {
		bot.log.Warning("Error getting response from reddit: %s", err)
		return ""
	}
	// Convert from JSON
	var raw_data interface{}
	if err := json.Unmarshal(body, &raw_data); err != nil {
		bot.log.Warning("Error parsing JSON from reddit: %s", err)
		return ""
	}
	data := raw_data.(map[string]interface{})
	if data["error"] != nil {
		bot.log.Debug("Reddit returned error %.0f.", data["error"])
		return ""
	} else {
		ext.announced[channel+url] = true
	}
	posts := data["data"].(map[string]interface{})["children"].([]interface{})
	message := ""
	best_score := 0
	for i := range posts {
		post_data := posts[i].(map[string]interface{})["data"].(map[string]interface{})
		score := int(post_data["score"].(float64))
		if score > best_score {
			title := post_data["title"].(string)
			if len(title) > 100 {
				title = title[:100] + "…"
			}
			msgData := &map[string]string{
				"elapsed":   HumanizedSince(time.Unix(int64(post_data["created"].(float64)), 0)),
				"name":      post_data["author"].(string),
				"subreddit": post_data["subreddit"].(string),
				"score":     fmt.Sprintf("%d", score),
				"comments":  "http://redd.it/" + post_data["id"].(string),
				"title":     title,
			}
			message = Format(ext.Texts.TempRedditAnnounce, msgData)
			best_score = score
		}
	}
	bot.log.Debug("Reddit: %s", message)
	return message
}

// ProcessURL will try to check if link was ever on reddit.
func (ext *ExtensionReddit) ProcessURL(bot *Bot, urlinfo *UrlInfo, channel, sender, msg string) {
	// Announce each link only once.
	if ext.announced[channel+urlinfo.URL] {
		return
	}

	// Can we fit into the ShortInfo?
	if urlinfo.ShortInfo == "" {
		urlinfo.ShortInfo = ext.getRedditInfo(bot, urlinfo.URL, channel)
	} else if len(urlinfo.ShortInfo) < 30 {
		reddit := ext.getRedditInfo(bot, urlinfo.URL, channel)
		if reddit != "" {
			urlinfo.ShortInfo += " | " + reddit
		}
	} else { // Better send as separate notcie.
		go func() {
			reddit := ext.getRedditInfo(bot, urlinfo.URL, channel)
			if reddit != "" {
				bot.SendNotice(channel, reddit)
			}
		}()
	}
}
