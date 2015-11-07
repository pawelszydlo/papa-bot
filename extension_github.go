package papaBot

import (
	"encoding/json"
	"fmt"
	"regexp"
)

// ExtensionGitHub - extension for getting basic repository information.
type ExtensionGitHub struct {
	gitHubRe *regexp.Regexp
}

// Init inits the extension.
func (ext *ExtensionGitHub) Init(bot *Bot) error {
	ext.gitHubRe = regexp.MustCompile(`(?i)github\.com/(.+?)/(.+?)(/|$)`)
	return nil
}

// ProcessURL will try to get more info on GitHub links.
func (ext *ExtensionGitHub) ProcessURL(bot *Bot, urlinfo *UrlInfo, channel, sender, msg string) {
	match := ext.gitHubRe.FindStringSubmatch(urlinfo.URL)
	if len(match) < 2 {
		return
	}
	// We will not be touching the shortInfo. Let's risk async get, maybe no one will ask for more in the meantime.
	urlinfo.LongInfo = "Loading data..." // Set some longInfo so that a marker will be placed on the notice
	go func() {
		user := match[1]
		repo := match[2]
		// Get response
		body, err := bot.getPageBodyByURL(fmt.Sprintf("https://api.github.com/repos/%s/%s", user, repo))
		if err != nil {
			bot.log.Warning("Error getting response from GitHub: %s", err)
			return
		}
		// Convert from JSON
		var raw_data interface{}
		if err := json.Unmarshal(body, &raw_data); err != nil {
			bot.log.Warning("Error parsing JSON from GitHub: %s", err)
			return
		}
		data := raw_data.(map[string]interface{})
		bot.urlMoreInfo[channel] = fmt.Sprintf(
			"%s: %s (created %s)\nForks: %.0f, stars: %.0f, subscribers: %.0f, open issues: %.0f",
			data["full_name"], data["description"], data["created_at"], data["forks_count"], data["stargazers_count"],
			data["subscribers_count"], data["open_issues_count"])
	}()
}

// Not implemented.
func (ext *ExtensionGitHub) Tick(bot *Bot, daily bool) {}
