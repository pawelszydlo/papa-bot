package papaBot

import (
	"encoding/json"
	"fmt"
	"github.com/pawelszydlo/papa-bot/utils"
	"net/url"
	"regexp"
	"strings"
)

// extensionWiki - finds Wikipedia articles.
type extensionWiki struct {
	Extension
	announced map[string]bool
	Texts     *extensionWikiTexts
}

type extensionWikiTexts struct {
	DidYouMean string
}

// Init inits the extension.
func (ext *extensionWiki) Init(bot *Bot) error {
	// Load texts.
	texts := &extensionWikiTexts{}
	if err := bot.LoadTexts(bot.textsFile, texts); err != nil {
		return err
	}
	ext.Texts = texts

	ext.announced = map[string]bool{}
	// Register new command.
	bot.MustRegisterCommand(&BotCommand{
		[]string{"w", "wiki"},
		false, false, false,
		"<article>", "Search wikipedia for <article>.",
		ext.commandWiki})
	return nil
}

// cleanWikiBody will try to remove as much distractions from an article as possible.
func (ext *extensionWiki) cleanWikiBody(content string) string {
	// Clean some distracting parts, like {{ }}.
	cleanupRe := regexp.MustCompile(`\{\{[^\{]*?\}\}|<ref.*?ref>`)
	original := content
	for {
		content = cleanupRe.ReplaceAllString(content, "")
		if original == content {
			break
		}
		original = content
	}
	// Remove everything before the first '''
	parts := strings.SplitN(content, "'''", 2)
	if len(parts) == 2 {
		content = parts[1]
	}
	// Remove special characters.
	content = strings.Replace(content, "'''", "", -1)
	content = strings.Replace(content, "''", "", -1)
	// Parse links.
	linkRe := regexp.MustCompile(`\[\[[^\[\]]+?\|(.+?)\]\]|\[\[([^\[\]]+?)\]\]`)
	content = linkRe.ReplaceAllStringFunc(content, func(text string) string {
		match := linkRe.FindStringSubmatch(text)
		if match[1] != "" {
			return match[1]
		}
		return match[2]
	})

	return content
}

// searchWiki will query Wikipedia database for information.
func (ext *extensionWiki) searchWiki(bot *Bot, lang, search string) (string, string) {
	// Fetch data.
	body, err := bot.GetPageBodyByURL(
		fmt.Sprintf(
			"http://%s.wikipedia.org/w/api.php?action=query&prop=revisions&format=json&rvprop=content&rvlimit=1"+
				"&rvsection=0&generator=search&redirects=&gsrwhat=text&gsrlimit=1&gsrsearch=%s",
			lang, url.QueryEscape(search),
		))
	if err != nil {
		bot.log.Warning("Error getting wiki data: %s", err)
		return "", ""
	}

	// Convert from JSON
	var raw_data interface{}
	if err := json.Unmarshal(body, &raw_data); err != nil {
		bot.log.Warning("Error parsing wiki data: %s", err)
		return "", ""
	}
	// Hacky digging.
	data := raw_data.(map[string]interface{})
	if data["query"] == nil {
		return "", ""
	}
	data = data["query"].(map[string]interface{})
	if data["pages"] == nil {
		return "", ""
	}
	data = data["pages"].(map[string]interface{})
	for _, page := range data {
		data := page.(map[string]interface{})
		if data["revisions"] == nil {
			return "", ""
		}
		title := data["title"].(string)
		revisions := data["revisions"].([]interface{})
		if len(revisions) == 0 {
			return "", ""
		}
		if revisions[0].(map[string]interface{})["*"] == nil {
			return "", ""
		}
		content := revisions[0].(map[string]interface{})["*"].(string)

		// Cleanup.M
		content = utils.StripTags(ext.cleanWikiBody(utils.CleanString(content, true)))
		title = utils.CleanString(title, true)

		return title, content
	}

	return "", ""
}

// commandMovie is a command for manually searching for movies.
func (ext *extensionWiki) commandWiki(bot *Bot, nick, user, channel, receiver string, priv bool, params []string) {
	if len(params) < 1 {
		return
	}
	search := strings.Join(params, " ")

	// Announce each article only once.
	if ext.announced[channel+search] {
		return
	}

	_, content := ext.searchWiki(bot, bot.Config.Language, search)

	// Announce.
	contentPreview := content
	contentFull := ""
	if len(content) > 300 {
		contentPreview = content[:300] + "…"
		contentFull = content
	}

	notice := fmt.Sprintf("%s, %s", nick, contentPreview)
	bot.SendPrivMessage(receiver, notice)
	ext.announced[channel+search] = true

	if contentFull != "" {
		bot.urlMoreInfo[receiver] = contentFull
	}
}
