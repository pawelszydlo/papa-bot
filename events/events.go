package events

// Events and dispatcher.

import (
	"github.com/sirupsen/logrus"
)

type EventCode int

// Single event codes.
const (
	// Normal chat message.
	EventChatMessage EventCode = iota
	// Chat notice.
	EventChatNotice EventCode = iota
	// Private message received.
	EventPrivateMessage EventCode = iota
	// URL found in the message.
	EventURLFound EventCode = iota

	// Transport connected.
	EventConnected EventCode = iota
	// Someone joined a channel.
	EventJoinedChannel EventCode = iota
	// Bot re-joined a channel.
	EventReJoinedChannel EventCode = iota
	// Someone left a channel.
	EventPartChannel EventCode = iota
	// Someone was kicked from a channel.
	EventKickedFromChannel EventCode = iota
	// Someone was banned from a channel.
	EventBannedFromChannel EventCode = iota
	// Other channel operations.
	EventChannelOps EventCode = iota

	// Bot tick.
	EventTick EventCode = iota
	// Daily bot tick.
	EventDailyTick EventCode = iota
)

// Event code groups, for convenience.
var EventsChannelActivity = []EventCode{
	EventChannelOps, EventJoinedChannel, EventReJoinedChannel, EventPartChannel, EventKickedFromChannel,
	EventBannedFromChannel}
var EventsChannelMessages = []EventCode{EventChatNotice, EventChatMessage}

// Message for the events channel.
type EventMessage struct {
	// Name of the transport that triggered the event.
	SourceTransport string
	// Event code.
	EventCode EventCode
	// Sender information
	Nick, FullName string
	Channel        string
	Message        string
	// Was the message directed at the bot? If yes, bot will check for commands.
	// In case of join, part etc. this will indicate whether bot was the subject.
	IsBot bool
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
	dispatcher.log.Debugf("Added listener for event \"%v\": %v", eventCode, listener)
}

// Trigger will trigger an event.
func (dispatcher *EventDispatcher) Trigger(eventMessage EventMessage) {
	if dispatcher.isIgnored(eventMessage) {
		dispatcher.log.Infof(
			"Ignoring event %v from %s (%s)", eventMessage.EventCode, eventMessage.Nick, eventMessage.FullName)
	}
	for _, listener := range dispatcher.listeners[eventMessage.EventCode] {
		go listener(eventMessage)
	}
}

// isIgnored will check whether the message comes from an ignored person.
func (dispatcher *EventDispatcher) isIgnored(eventMessage EventMessage) bool {
	for _, person := range dispatcher.blackList {
		if person == eventMessage.FullName {
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