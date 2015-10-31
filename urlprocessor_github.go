package papaBot

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"regexp"
)

var (
	gitHubRe = regexp.MustCompile(`(?i)github\.com/(.*?)/(.*?)(/|$)`)
)

// Initializes stuff needed by this processor.
func initUrlProcessorGithub(bot *Bot) {
}

func urlProcessorGithub(bot *Bot, info *urlInfo, channel, sender, msg string) {
	match := gitHubRe.FindStringSubmatch(info.link)
	if len(match) < 2 {
		return
	}
	user := match[1]
	repo := match[2]
	// Get response
	resp, err := GetHTTPResponse(fmt.Sprintf("https://api.github.com/repos/%s/%s", user, repo))
	if err != nil {
		bot.log.Debug("Error getting response from GitHub: %s", err)
		return
	}
	defer resp.Body.Close()

	// Load body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		bot.log.Debug("Error loading body from GitHub: %s", err)
		return
	}
	var raw_data interface{}
	if err := json.Unmarshal(body, &raw_data); err != nil {
		bot.log.Debug("Error decoding JSON from GitHub: %s", err)
		return
	}
	bot.log.Debug("Received data from GitHub API.")
	data := raw_data.(map[string]interface{})
	info.longInfo = fmt.Sprintf(
		"%s: %s (created %s)\nForks: %.0f, stars: %.0f, subscribers: %.0f, open issues: %.0f",
		data["full_name"], data["description"], data["created_at"], data["forks_count"], data["stargazers_count"],
		data["subscribers_count"], data["open_issues_count"])
}
