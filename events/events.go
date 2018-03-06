package events

// Events and dispatcher.
//go:generate stringer -type=EventCode

import (
	"github.com/sirupsen/logrus"
)

type EventCode int

// Single event codes.
const (
	// Normal chat message.
	EventChatMessage EventCode = iota
	// Chat notice.
	EventChatNotice
	// Private message received.
	EventPrivateMessage
	// URL found in the message.
	EventURLFound

	// Bot activity, like command or event handler started.
	EventBotWorking
	// Bot finished activity.
	EventBotDone

	// Transport connected.
	EventConnected
	// Someone joined a channel.
	EventJoinedChannel
	// Bot re-joined a channel.
	EventReJoinedChannel
	// Someone left a channel.
	EventPartChannel
	// Someone was kicked from a channel.
	EventKickedFromChannel
	// Someone was banned from a channel.
	EventBannedFromChannel
	// Other channel operations.
	EventChannelOps

	// Bot tick.
	EventTick
	// Daily bot tick.
	EventDailyTick
)

// Event code groups, for convenience.
var EventsChannelActivity = []EventCode{
	EventChannelOps, EventJoinedChannel, EventReJoinedChannel, EventPartChannel, EventKickedFromChannel,
	EventBannedFromChannel}
var EventsChannelMessages = []EventCode{EventChatNotice, EventChatMessage}

// Message formatting options.
type Formatting int
const (
	FormatPlain Formatting = iota
	FormatIRC
	FormatMarkdown
)

// Message for the events channel.
type EventMessage struct {
	// Name of the transport that triggered the event.
	TransportName string
	// Message formatting accepted by the transport.
	TransportFormatting Formatting
	// Event code.
	EventCode EventCode
	// Sender information
	Nick, UserId string
	Channel        string
	Message        string
	// Context for the message, will be passed back if any listener sends a message.
	Context string
	// Was the message directed at the bot? If yes, bot will check for commands.
	// Message directed at the bot should be stripped of the prefixes like dot or bot's name.
	// In case of join, part etc. this will indicate whether bot was the subject.
	AtBot bool
}

// IsPrivate will tell if an event was triggered by a private chat message.
func (message *EventMessage) IsPrivate() bool {
	if message.EventCode == EventPrivateMessage {
		return true
	}
	return false
}

// Type for a valid event listener function.
type EventListenerFunc func(message EventMessage)

// Event dispatcher.
type EventDispatcher struct {
	listeners map[EventCode][]EventListenerFunc
	log       *logrus.Logger
	// List of people whos events will be ignored, in the form of transport~nick.
	blackList []string
}

// RegisterMultiListener will attach a listener to multiple events.
func (dispatcher *EventDispatcher) RegisterMultiListener(eventCodes []EventCode, listener EventListenerFunc) {
	for _, eventCode := range eventCodes {
		dispatcher.RegisterListener(eventCode, listener)
	}
}

// RegisterListener will register a listener to an event.
func (dispatcher *EventDispatcher) RegisterListener(eventCode EventCode, listener EventListenerFunc) {
	// RegisterExtension will register a new extension with the bot.
	dispatcher.listeners[eventCode] = append(dispatcher.listeners[eventCode], listener)
	dispatcher.log.Debugf("Added listener for event \"%s\": %v", eventCode, listener)
}

// Trigger will trigger an event.
func (dispatcher *EventDispatcher) Trigger(eventMessage EventMessage) {
	if dispatcher.isIgnored(eventMessage) {
		dispatcher.log.Infof(
			"Ignoring event %s from %s (%s)", eventMessage.EventCode, eventMessage.Nick, eventMessage.UserId)
		return
	}
	for _, listener := range dispatcher.listeners[eventMessage.EventCode] {
		go func(listener EventListenerFunc) {
			// Catch errors.
			defer func() {
				if r := recover(); r != nil {
					dispatcher.log.Errorf("FATAL ERROR in event handler for %v: %v", eventMessage.EventCode, r)
				}
			}()
			listener(eventMessage)
		}(listener)
	}
}

// isIgnored will check whether the message comes from an ignored person.
func (dispatcher *EventDispatcher) isIgnored(eventMessage EventMessage) bool {
	if eventMessage.UserId == "" {
		return false
	}
	for _, person := range dispatcher.blackList {
		if person == eventMessage.UserId {
			return true
		}
	}
	return false
}

// SetBlackList sets the ignore list.
func (dispatcher *EventDispatcher) SetBlackList(blackList []string) {
	dispatcher.blackList = blackList
}

// New will create a new event dispatcher instance.
func New(logger *logrus.Logger) *EventDispatcher {
	dispatcher := &EventDispatcher{
		listeners: map[EventCode][]EventListenerFunc{},
		log:       logger,
	}
	return dispatcher
}
