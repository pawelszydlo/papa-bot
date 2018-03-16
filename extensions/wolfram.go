package extensions

import (
	"encoding/xml"
	"fmt"
	"github.com/pawelszydlo/papa-bot"
	"github.com/pawelszydlo/papa-bot/events"
	"net/url"
	"strings"
)

/*
ExtensionWolfram - query Wolfram Alpha.

Used custom variables:
- WolframKey - Your Wolfram Alpha AppID key.
*/
type ExtensionWolfram struct {
	announced map[string]string
	Texts     *extensionWolframTexts
	bot       *papaBot.Bot
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
	ext.announced = map[string]string{}
	// Load texts.
	texts := &extensionWolframTexts{}
	if err := bot.LoadTexts("wolfram", texts); err != nil {
		return err
	}
	ext.Texts = texts
	// Register new command.
	bot.RegisterCommand(&papaBot.BotCommand{
		[]string{"wa", "wolfram"},
		false, false, false,
		"<query>", "Search Wolfram Alpha for <query>.",
		ext.commandWolfram})
	ext.bot = bot
	return nil
}

func (ext *ExtensionWolfram) queryWolfram(query string, format events.Formatting) string {
	appId := ext.bot.GetVar("WolframKey")
	if appId == "" {
		ext.bot.Log.Error("Wolfram Alpha AppID key not set! Set the 'WolframKey' variable in the bot.")
		return ""
	}

	ext.bot.Log.Debugf("Querying WolframAlpha for %s...", query)
	err, _, body := ext.bot.GetPageBody(
		fmt.Sprintf(
			"http://api.wolframalpha.com/v2/query?format=plaintext&appid=%s&podindex=1,2&input=%s",
			appId, strings.Replace(url.QueryEscape(query), "+", "%20", -1),
		), nil)
	if err != nil {
		ext.bot.Log.Warningf("Error getting Wolfram data: %s", err)
		return ""
	}

	// Parse XML.
	data := new(queryResult)
	if err := xml.Unmarshal(body, &data); err != nil {
		ext.bot.Log.Errorf("Error parsing Wolfram data: %s", err)
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
				if format == events.FormatMarkdown {
					result = append(result, "**"+subpod.PlainText+"**")
				} else if format == events.FormatIRC {
					result = append(result, "\x0308"+subpod.PlainText+"\x03")
				} else {
					result = append(result, subpod.PlainText)
				}
			}
		}
	}

	return fmt.Sprintf("%s = %s", input, strings.Join(result, ext.Texts.Or))
}

// commandMovie is a command for manually searching for movies.
func (ext *ExtensionWolfram) commandWolfram(bot *papaBot.Bot, sourceEvent *events.EventMessage, params []string) {
	if len(params) < 1 {
		return
	}
	search := strings.Join(params, " ")

	// Check if we have the result cached.
	if val, exists := ext.announced[sourceEvent.Channel+search]; exists {
		bot.SendMessage(sourceEvent, fmt.Sprintf("%s, %s", sourceEvent.Nick, val))
		return
	}

	content := ext.queryWolfram(search, sourceEvent.TransportFormatting)

	// Error occured.
	if content == "" {
		return
	}

	maxLen := 300
	if sourceEvent.TransportFormatting == events.FormatMarkdown {
		maxLen = 3000
	}

	// Announce.
	contentPreview := content
	contentFull := ""
	if len(content) > maxLen {
		contentPreview = content[:maxLen] + "…"
		contentFull = content
	}

	bot.SendMessage(sourceEvent, fmt.Sprintf("%s, %s", sourceEvent.Nick, contentPreview))
	ext.announced[sourceEvent.Channel+search] = contentPreview

	if contentFull != "" {
		bot.AddMoreInfo(sourceEvent.TransportName, sourceEvent.Channel, contentFull)
	}
}
