package extensions

import (
	"github.com/pawelszydlo/papa-bot"
	"github.com/pawelszydlo/papa-bot/events"
	"math/rand"
)

// ExtensionTalk makes the bot a bit more talkative.
type ExtensionTalk struct {
	Texts *extensionTalkTexts
	bot   *papaBot.Bot
}

type extensionTalkTexts struct {
	Hellos          []string
	HellosAfterKick []string
}

// Init inits the extension.
func (ext *ExtensionTalk) Init(bot *papaBot.Bot) error {
	texts := new(extensionTalkTexts) // Can't load directly because of reflection issues.
	if err := bot.LoadTexts("talk", texts); err != nil {
		return err
	}
	ext.Texts = texts
	ext.bot = bot
	bot.EventDispatcher.RegisterListener(events.EventJoinedChannel, ext.JoinedListener)
	bot.EventDispatcher.RegisterListener(events.EventReJoinedChannel, ext.ReJoinedListener)
	return nil
}

// JoinedListener says something when bot joins a channel.
func (ext *ExtensionTalk) JoinedListener(message events.EventMessage) {
	if !message.AtBot || message.Transport == "mattermost" {
		return
	}
	ext.bot.SendMessage(&message, ext.Texts.Hellos[rand.Intn(len(ext.Texts.Hellos))])
}

// ReJoinedListener says something when bot joins a channel.
func (ext *ExtensionTalk) ReJoinedListener(message events.EventMessage) {
	if !message.AtBot {
		return
	}
	ext.bot.SendMessage(&message, ext.Texts.HellosAfterKick[rand.Intn(len(ext.Texts.HellosAfterKick))])
}
