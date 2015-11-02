package papaBot

import (
	"regexp"
	"strings"
)

// ExtensionMeta - extension for getting title and description from html links.
type ExtensionMeta struct {
	titleRe, metaRe, descRe *regexp.Regexp
}

// Init inits the extension.
func (ext *ExtensionMeta) Init(bot *Bot) error {
	ext.titleRe = regexp.MustCompile("(?is)<title.*?>(.+?)</title>")
	ext.metaRe = regexp.MustCompile(`(?is)<\s*?meta.*?content\s*?=\s*?"(.*?)".*?>`)
	ext.descRe = regexp.MustCompile(`(?is)(property|name)\s*?=.*?description`)
	return nil
}

// getTitle find the title and description.
func (ext *ExtensionMeta) getTitle(body string) (string, string, error) {
	// Iterate over meta tags to get the description
	description := ""
	metas := ext.metaRe.FindAllStringSubmatch(string(body), -1)
	for i := range metas {
		if len(metas[i]) > 1 {
			isDesc := ext.descRe.FindString(metas[i][0])
			if isDesc != "" && (len(metas[i][1]) > len(description)) {
				description = CleanString(metas[i][1])
			}
		}
	}
	// Get the title
	match := ext.titleRe.FindStringSubmatch(string(body))
	if len(match) > 1 {
		title := CleanString(match[1])
		return title, description, nil
	}

	return "", "", nil
}

// ProcessURL will try to get the title and description.
func (ext *ExtensionMeta) ProcessURL(bot *Bot, urlinfo *UrlInfo, channel, sender, msg string) {
	if len(urlinfo.Body) == 0 || !strings.Contains(urlinfo.ContentType, "html") {
		return
	}
	// Get the title
	title, description, err := ext.getTitle(string(urlinfo.Body))
	if err != nil {
		bot.log.Warning("Error getting title: %s", err)
	} else {
		bot.log.Debug("Title: %s", title)
	}
	urlinfo.Title = title
	urlinfo.ShortInfo = title
	urlinfo.LongInfo = description
}
