package extensions

import (
	"encoding/json"
	"fmt"
	"github.com/pawelszydlo/papa-bot"
	"github.com/pawelszydlo/papa-bot/events"
	"net/url"
	"regexp"
	"strings"
)

// ExtensionWiki - finds Wikipedia articles.
type ExtensionWiki struct {
	announced map[string]bool
	linkRe    *regexp.Regexp
	cleanupRe *regexp.Regexp
	Texts     *extensionWikiTexts
	bot       *papaBot.Bot
}

type extensionWikiTexts struct {
	DidYouMean string
}

type WikiSearchResult struct {
	Batchcomplete string `json:"batchcomplete"`
	Query         struct {
		Search []struct {
			Ns     int    `json:"ns"`
			Title  string `json:"title"`
			Pageid int    `json:"pageid"`
		} `json:"search"`
	} `json:"query"`
}

type WikiPage struct {
	Batchcomplete bool `json:"batchcomplete"`
	Query         struct {
		Normalized []struct {
			Fromencoded bool   `json:"fromencoded"`
			From        string `json:"from"`
			To          string `json:"to"`
		} `json:"normalized"`
		Pages []struct {
			Pageid  int    `json:"pageid"`
			Ns      int    `json:"ns"`
			Title   string `json:"title"`
			Extract string `json:"extract"`
			Missing bool   `json:"missing"`
		} `json:"pages"`
	} `json:"query"`
}

// Init inits the extension.
func (ext *ExtensionWiki) Init(bot *papaBot.Bot) error {
	// Load texts.
	texts := &extensionWikiTexts{}
	if err := bot.LoadTexts("wiki", texts); err != nil {
		return err
	}
	ext.Texts = texts
	// Init variables.
	ext.announced = map[string]bool{}
	ext.linkRe = regexp.MustCompile(`\[\[[^\[\]]+?\|(.+?)\]\]|\[\[([^\[\]]+?)\]\]`)
	ext.cleanupRe = regexp.MustCompile(`\{\{[^\{]*?\}\}|<ref.*?ref>`)
	// Register new command.
	bot.RegisterCommand(&papaBot.BotCommand{
		[]string{"w", "wiki"},
		false, false, false,
		"<article>", "Search wikipedia for <article>.",
		ext.commandWiki})
	ext.bot = bot
	return nil
}

// searchWiki will query Wikipedia database for information.
func (ext *ExtensionWiki) searchWiki(lang, search string) (string, string) {
	// Fetch search result data.
	err, _, body := ext.bot.GetPageBody(
		fmt.Sprintf(
			"https://%s.wikipedia.org/w/api.php?action=query&format=json&list=search&srlimit=1&srwhat=nearmatch" +
				"&srprop=&srenablerewrites=1&srsearch=%s",
			lang, url.QueryEscape(search),
		), nil)
	if err != nil {
		ext.bot.Log.Warningf("Error getting wiki search results: %s", err)
		return "", ""
	}

	// Convert result from JSON
	var search_result = WikiSearchResult{}
	if err := json.Unmarshal(body, &search_result); err != nil {
		ext.bot.Log.Warningf("Error parsing wiki search results: %s", err)
		return "", ""
	}

	if len(search_result.Query.Search) == 0 {
		return "", "¯\\\\_(ツ)_/¯"
	}

	// Fetch page data.
	err, _, body = ext.bot.GetPageBody(
		fmt.Sprintf(
			"https://%s.wikipedia.org/w/api.php?action=query&format=json&prop=extracts&utf8=1&formatversion=2" +
				"&exsentences=8&exlimit=1&explaintext=1&pageids=%d",
			lang, search_result.Query.Search[0].Pageid,
		), nil)
	if err != nil {
		ext.bot.Log.Warningf("Error getting wiki page: %s", err)
		return "", ""
	}

	// Convert page from JSON
	var page_result = WikiPage{}
	if err := json.Unmarshal(body, &page_result); err != nil {
		ext.bot.Log.Warningf("Error parsing wiki page: %s", err)
		return "", ""
	}

	if len(page_result.Query.Pages) == 0 || page_result.Query.Pages[0].Pageid == 0 {
		return "", "¯\\\\_(ツ)_/¯"
	}

	return "", page_result.Query.Pages[0].Extract
}

// commandWiki is a command for manually searching for wikipedia entries.
func (ext *ExtensionWiki) commandWiki(bot *papaBot.Bot, sourceEvent *events.EventMessage, params []string) {
	if len(params) < 1 {
		return
	}
	search := strings.Join(params, " ")

	// Announce each article only once.
	if ext.announced[sourceEvent.ChannelId()+search] {
		return
	}

	_, content := ext.searchWiki(bot.Config.Language, search)

	maxLen := 300
	if sourceEvent.TransportName == "mattermost" {
		maxLen = 3000
	}

	// Announce.
	contentPreview := content
	contentFull := ""
	if len(content) > maxLen {
		contentPreview = content[:maxLen] + "…"
		contentFull = content
	}

	notice := fmt.Sprintf("%s, %s", sourceEvent.Nick, contentPreview)
	bot.SendMessage(sourceEvent, notice)
	ext.announced[sourceEvent.ChannelId()+search] = true

	if contentFull != "" {
		bot.AddMoreInfo(sourceEvent.TransportName, sourceEvent.Channel, contentFull)
	}
}
