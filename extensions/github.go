package extensions

import (
	"encoding/json"
	"fmt"
	"github.com/pawelszydlo/papa-bot"
	"github.com/pawelszydlo/papa-bot/events"
	"regexp"
)

// ExtensionGitHub - extension for getting basic repository information.
type ExtensionGitHub struct {
	Extension
	gitHubRe *regexp.Regexp
	bot      *papaBot.Bot
}

// Init inits the extension.
func (ext *ExtensionGitHub) Init(bot *papaBot.Bot) error {
	ext.gitHubRe = regexp.MustCompile(`(?i)github\.com/(.+?)/(.+?)(/|$)`)
	ext.bot = bot
	bot.EventDispatcher.RegisterListener(events.EventURLFound, ext.UrlListener)
	return nil
}

// UrlListener will try to get more info on GitHub links.
func (ext *ExtensionGitHub) UrlListener(message events.EventMessage) {
	match := ext.gitHubRe.FindStringSubmatch(message.Message)
	if len(match) < 2 {
		return
	}
	go func() {
		user := match[1]
		repo := match[2]
		// Get response
		err, _, body := ext.bot.GetPageBody(fmt.Sprintf("https://api.github.com/repos/%s/%s", user, repo), nil)
		if err != nil {
			ext.bot.Log.Warningf("Error getting response from GitHub: %s", err)
			return
		}
		// Convert from JSON
		var raw_data interface{}
		if err := json.Unmarshal(body, &raw_data); err != nil {
			ext.bot.Log.Warningf("Error parsing JSON from GitHub: %s", err)
			return
		}
		data := raw_data.(map[string]interface{})
		ext.bot.AddMoreInfo(message.SourceTransport, message.Channel, fmt.Sprintf(
			"%s: %s (created %s)\nForks: %.0f, stars: %.0f, subscribers: %.0f, open issues: %.0f",
			data["full_name"], data["description"], data["created_at"], data["forks_count"], data["stargazers_count"],
			data["subscribers_count"], data["open_issues_count"]))
	}()
}
