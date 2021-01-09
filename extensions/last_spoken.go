package extensions

import (
	"encoding/json"
	"github.com/pawelszydlo/papa-bot"
	"github.com/pawelszydlo/papa-bot/events"
	"github.com/pawelszydlo/papa-bot/utils"
	"strings"
	"text/template"
	"time"
)

// ExtensionLastSpoken tracks when a person has last spoken.
type ExtensionLastSpoken struct {
	LastAsk    map[string]map[string]time.Time
	LastSpoken map[string]map[string]time.Time

	Texts *ExtensionLastSpokenTexts
	bot   *papaBot.Bot
}

type ExtensionLastSpokenTexts struct {
	NeverSpoken    string
	TempLastSpoken *template.Template
}

// Init inits the extension.
func (ext *ExtensionLastSpoken) Init(bot *papaBot.Bot) error {
	// Register new command.
	bot.RegisterCommand(&papaBot.BotCommand{
		[]string{"ls", "lastspoken"},
		false, false, false,
		"<nick>", "Show when the person was last seen speaking.",
		ext.commandLastSpoken})
	// Load texts.
	texts := new(ExtensionLastSpokenTexts) // Can't load directly because of reflection issues.
	if err := bot.LoadTexts("last_spoken", texts); err != nil {
		return err
	}
	ext.Texts = texts
	ext.bot = bot
	// Init first level maps.
	ext.LastAsk = map[string]map[string]time.Time{}
	ext.LastSpoken = map[string]map[string]time.Time{}
	// Attach event handler.
	bot.EventDispatcher.RegisterListener(events.EventChatMessage, ext.ChatListener)
	// Load previous data.
	if err := json.Unmarshal([]byte(bot.GetVar("LastSpoken")), &ext.LastSpoken); err != nil {
		ext.bot.Log.Warningf("Error parsing JSON from LastSpoken: %s", err)
	} else {
		ext.bot.Log.Infof("Successfully loaded previous LastSpoken data for %d channels.", len(ext.LastSpoken))
	}
	return nil
}

func (ext *ExtensionLastSpoken) saveLastSpoken() {
	jsonString, err := json.Marshal(ext.LastSpoken)
	if err != nil {
		ext.bot.Log.Errorf("Can't save LastSpoken data: %s", err)
	} else {
		ext.bot.Log.Infof("Saving last spoken data: %s", string(jsonString))
		ext.bot.SetVar("LastSpoken", string(jsonString))
	}
}

// ChatListener listens to chat messages and records who has spoken.
func (ext *ExtensionLastSpoken) ChatListener(message events.EventMessage) {
	// Make sure the channel map is initialized.
	if ext.LastSpoken[message.ChannelId()] == nil {
		ext.LastSpoken[message.ChannelId()] = map[string]time.Time{}
	}
	ext.LastSpoken[message.ChannelId()][message.Nick] = time.Now()
	ext.saveLastSpoken()
}

func (ext *ExtensionLastSpoken) commandLastSpoken(bot *papaBot.Bot, sourceEvent *events.EventMessage, params []string) {
	if len(params) < 1 {
		return
	}
	nick := strings.Join(params, " ")
	// Make sure the channel maps are initialized.
	if ext.LastAsk[sourceEvent.ChannelId()] == nil {
		ext.LastAsk[sourceEvent.ChannelId()] = map[string]time.Time{}
	}
	if ext.LastSpoken[sourceEvent.ChannelId()] == nil {
		ext.LastSpoken[sourceEvent.ChannelId()] = map[string]time.Time{}
	}
	// Answer only once per 5 minutes per channel.
	if time.Since(ext.LastAsk[sourceEvent.ChannelId()][nick]) > 5*time.Minute {
		message := ext.Texts.NeverSpoken
		ext.LastAsk[sourceEvent.ChannelId()][nick] = time.Now()
		if !ext.LastSpoken[sourceEvent.ChannelId()][nick].IsZero() {
			message = utils.Format(ext.Texts.TempLastSpoken, map[string]string{
				"heard": ext.bot.Humanizer.TimeDiffNow(
					utils.MustForceLocalTimezone(ext.LastSpoken[sourceEvent.ChannelId()][nick]), true),
				"nick": nick,
			})
		}
		bot.SendMessage(sourceEvent, message)
	}
}
