package extensions

import (
	"encoding/xml"
	"fmt"
	"github.com/pawelszydlo/papa-bot"
	"net/url"
	"strings"
)

/*
ExtensionWolfram - query Wolfram Alpha.

Used custom variables:
- WolframKey - Your Wolfram Alpha AppID key.
*/
type ExtensionWolfram struct {
	Extension
	announced map[string]bool
	Texts     *extensionWolframTexts
}

type extensionWolframTexts struct {
	NoResult string
	Or       string
}

// Structs for Wolfram responses.
type queryResult struct {
	Success bool  `xml:"success,attr"`
	Error   bool  `xml:"error,attr"`
	Pods    []pod `xml:"pod"`
}

type pod struct {
	Title   string `xml:"title,attr"`
	Scanner string `xml:"scanner,attr"`
	Id      string `xml:"id,attr"`

	Subpods []subPod `xml:"subpod"`
}

type subPod struct {
	Title     string `xml:"title,attr"`
	PlainText string `xml:"plaintext"`
}

// Init inits the extension.
func (ext *ExtensionWolfram) Init(bot *papaBot.Bot) error {
	// Init variables.
	ext.announced = map[string]bool{}
	// Load texts.
	texts := &extensionWolframTexts{}
	if err := bot.LoadTexts(bot.TextsFile, texts); err != nil {
		return err
	}
	ext.Texts = texts
	// Register new command.
	bot.RegisterCommand(&papaBot.BotCommand{
		[]string{"wa", "wolfram"},
		false, false, false,
		"<query>", "Search Wolfram Alpha for <query>.",
		ext.commandWolfram})
	return nil
}

func (ext *ExtensionWolfram) queryWolfram(bot *papaBot.Bot, query string) string {
	appId := bot.GetVar("WolframKey")
	if appId == "" {
		bot.Log.Error("Wolfram Alpha AppID key not set! Set the 'WolframKey' variable in the bot.")
		return ""
	}

	body, err := bot.GetPageBodyByURL(
		fmt.Sprintf(
			"http://api.wolframalpha.com/v2/query?format=plaintext&appid=%s&podindex=1,2&input=%s",
			appId, strings.Replace(url.QueryEscape(query), "+", "%20", -1),
		))
	if err != nil {
		bot.Log.Warningf("Error getting Wolfram data: %s", err)
		return ""
	}

	// Parse XML.
	data := new(queryResult)
	if err := xml.Unmarshal(body, &data); err != nil {
		bot.Log.Errorf("Error parsing Wolfram data: %s", err)
		return ""
	}

	// Did the query succeed?
	if data.Error || !data.Success {
		return ext.Texts.NoResult
	}

	input := ""
	result := []string{} // Result can have multiple forms.

	for _, pod := range data.Pods {
		// Get the input interpretation.
		if pod.Id == "Input" {
			for _, subpod := range pod.Subpods {
				input = subpod.PlainText
			}
		}
		// Get the result.
		if pod.Id == "Result" || pod.Id == "Value" {
			for _, subpod := range pod.Subpods {
				result = append(result, "\x0308"+subpod.PlainText+"\x03")
			}
		}
	}

	return fmt.Sprintf("%s = %s", input, strings.Join(result, ext.Texts.Or))
}

// commandMovie is a command for manually searching for movies.
func (ext *ExtensionWolfram) commandWolfram(bot *papaBot.Bot, nick, user, channel, receiver string, priv bool, params []string) {
	if len(params) < 1 {
		return
	}
	search := strings.Join(params, " ")

	// Announce each article only once.
	if ext.announced[channel+search] {
		return
	}

	content := ext.queryWolfram(bot, search)

	// Error occured.
	if content == "" {
		return
	}

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
		bot.AddMoreInfo(receiver, contentFull)
	}
}
