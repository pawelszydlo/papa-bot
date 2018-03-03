package mattermostTransport

import (
	"github.com/mattermost/mattermost-server/model"
	"github.com/pawelszydlo/papa-bot/events"
	"strings"
)

// Interface for event handler function.
type eventHandlerFunc func(event *model.WebSocketEvent)

// registerEventHandler will register a new handler for the given event.
func (transport *MattermostTransport) registerEventHandler(event string, handler eventHandlerFunc) {
	transport.eventHandlers[event] = append(transport.eventHandlers[event], handler)
}

// registerAllEventHandlers will register all built-in handlers.
func (transport *MattermostTransport) registerAllEventHandlers() {
	transport.registerEventHandler(model.WEBSOCKET_EVENT_POSTED, transport.postedHandler)
	transport.registerEventHandler(model.WEBSOCKET_EVENT_HELLO, transport.helloHandler)
}

// postedHandler handles posted messages.
func (transport *MattermostTransport) postedHandler(event *model.WebSocketEvent) {
	post := model.PostFromJson(strings.NewReader(event.Data["post"].(string)))
	// Ignore bot events.
	if post.UserId == transport.mmUser.Id {
		return
	}
	// Did the message come from one of the channels bot is on?
	if transport.imOnChannel(post.ChannelId) {
		processedMsg, direct := transport.directMessage(post.Message)
		transport.sendEvent(
			events.EventChatMessage,
			direct,
			transport.onChannel[post.ChannelId],
			transport.userIdToNick(post.UserId),
			post.UserId,
			processedMsg)
	} else { // Some other message. Check, maybe it is a private chat.
		if channel, response := transport.client.GetChannel(post.ChannelId, ""); response.Error != nil {
			transport.log.Warnf("Couldn't get info for channel %s %s", post.ChannelId, response.Error)
		} else {
			if channel.Type == model.CHANNEL_DIRECT {
				processedMsg, _ := transport.directMessage(post.Message)
				// Add the channel to the ones bot is on.
				sender := transport.userIdToNick(post.UserId)
				transport.onChannel[channel.Id] = channel.Name
				transport.log.Warnf("Added new chanel: %s", channel.Name)
				transport.sendEvent(
					events.EventPrivateMessage,
					true,
					channel.Name,
					sender,
					post.UserId,
					processedMsg)
			}
		}
	}

}

// helloHandler joins the channels.
func (transport *MattermostTransport) helloHandler(event *model.WebSocketEvent) {
	transport.joinChannels()
}
