package extensions

import (
	"encoding/json"
	"fmt"
	"github.com/pawelszydlo/papa-bot"
	"github.com/pawelszydlo/papa-bot/events"
	"github.com/pawelszydlo/papa-bot/utils"
	"net/url"
	"regexp"
	"text/template"
	"time"
)

// ExtensionYoutube - extension for getting basic video information.
type ExtensionYoutube struct {
	youTubeRe *regexp.Regexp
	bot       *papaBot.Bot
	Texts     *ExtensionYoutubeTexts
}

type ExtensionYoutubeTexts struct {
	TempNotice *template.Template
}

// Init inits the extension.
func (ext *ExtensionYoutube) Init(bot *papaBot.Bot) error {
	ext.youTubeRe = regexp.MustCompile(`(?i)youtu(?:be\.com/watch\?v=|\.be/)([\w\-_]*)(&(amp;)?‌​[\w?‌​=]*)?`)
	ext.bot = bot
	// Load texts.
	texts := new(ExtensionYoutubeTexts)
	if err := bot.LoadTexts("youtube", texts); err != nil {
		return err
	}
	ext.Texts = texts
	bot.EventDispatcher.RegisterListener(events.EventURLFound, ext.UrlListener)
	return nil
}

// UrlListener will try to get more info on GitHub links.
func (ext *ExtensionYoutube) UrlListener(message events.EventMessage) {
	match := ext.youTubeRe.FindStringSubmatch(message.Message)
	if len(match) < 2 {
		return
	}
	go func() {
		video_no := match[1]
		// Get response
		err, _, body := ext.bot.GetPageBody(fmt.Sprintf("https://youtube.com/get_video_info?video_id=%s", video_no), nil)
		if err != nil {
			ext.bot.Log.Warningf("Error getting response from YouTube: %s", err)
			return
		}
		// Extract data from www-from-urlencoded.
		params, err := url.ParseQuery(string(body))
		if err != nil {
			ext.bot.Log.Error(err)
			return
		}
		// Intersting stuff is only in the "player_response" -> "videoDetails".
		response, ok := params["player_response"]
		if !ok {
			ext.bot.Log.Error("Player response not found.")
		}
		// Convert from JSON
		var raw_data interface{}
		if err := json.Unmarshal([]byte(response[0]), &raw_data); err != nil {
			ext.bot.Log.Warningf("Error parsing JSON from YouTube get info: %s", err)
			return
		}
		data := raw_data.(map[string]interface{})["videoDetails"].(map[string]interface{})

		// Map that the user will be able to use for formatting.
		duration, err := time.ParseDuration(fmt.Sprintf("%ss", data["lengthSeconds"]))
		values := map[string]string{
			"title":       fmt.Sprintf("%s", data["title"]),
			"length":      utils.FormatDuration(duration),
			"description": fmt.Sprintf("%s", data["shortDescription"]),
			"rating":      fmt.Sprintf("%.2f", data["averageRating"]),
			"views":       fmt.Sprintf("%s", data["viewCount"]),
			"author":      fmt.Sprintf("%s", data["author"]),
		}

		// Add "more".
		ext.bot.AddMoreInfo(message.TransportName, message.Channel, values["description"])

		// Send the notice.
		ext.bot.SendNotice(&message, utils.Format(ext.Texts.TempNotice, values))
	}()
}
