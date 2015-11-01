package papaBot

import (
	"fmt"
	"regexp"
)

type UrlProcessorGitHub struct {
	gitHubRe *regexp.Regexp
}

// Init the processor.
func (proc *UrlProcessorGitHub) Init(bot *Bot) error {
	proc.gitHubRe = regexp.MustCompile(`(?i)github\.com/(.*?)/(.*?)(/|$)`)
	return nil
}

// Processor will try to get more info on GitHub links.
func (proc *UrlProcessorGitHub) Process(bot *Bot, info *urlInfo, channel, sender, msg string) {
	match := proc.gitHubRe.FindStringSubmatch(info.link)
	if len(match) < 2 {
		return
	}
	// We will not be touching the shortInfo. Let's risk async get, maybe no one will ask for more in the meantime.
	info.longInfo = "Loading data..." // Set some longInfo so that a marker will be placed on the notice
	go func() {
		user := match[1]
		repo := match[2]
		// Get response
		data, err := GetJsonResponse(fmt.Sprintf("https://api.github.com/repos/%s/%s", user, repo))
		if err != nil {
			bot.log.Debug("Error getting response from GitHub: %s", err)
			return
		}
		bot.urlMoreInfo[channel] = fmt.Sprintf(
			"%s: %s (created %s)\nForks: %.0f, stars: %.0f, subscribers: %.0f, open issues: %.0f",
			data["full_name"], data["description"], data["created_at"], data["forks_count"], data["stargazers_count"],
			data["subscribers_count"], data["open_issues_count"])
	}()
}
