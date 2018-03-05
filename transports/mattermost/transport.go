package mattermostTransport

import (
	"fmt"
	"github.com/mattermost/mattermost-server/model"
	"github.com/pawelszydlo/papa-bot/events"
	"github.com/pawelszydlo/papa-bot/utils"
	"github.com/pelletier/go-toml"
	"github.com/sirupsen/logrus"
	"strings"
	"time"
)

type MattermostTransport struct {
	// Settings.

	// Connection parameters
	server    string
	websocket string
	user      string // Transport logs in by username. Ideally this will be the bot's name.
	password  string
	team      string
	// channels to join.
	channels []string
	// Delay of next messages after flood semaphore hits.
	antiFloodDelay int
	// Delay between rejoin attempts.
	rejoinDelay time.Duration
	// Bot info.
	botName string

	// Provided by the bot.

	// Logger.
	log *logrus.Logger
	// Scribe channel
	eventDispatcher *events.EventDispatcher

	// Operational.

	// Mattermost client
	client *model.Client4
	// Mattermost identity.
	mmUser *model.User
	mmTeam *model.Team
	// Websocket client.
	webSocketClient *model.WebSocketClient
	// Channels the bot is on. channelId -> name
	onChannel map[string]*model.Channel
	// Registered event handlers.
	eventHandlers map[string][]eventHandlerFunc
	// User identification cache userId -> nick
	users map[string]string
}

// Init initializes a transport instance.
func (transport *MattermostTransport) Init(botName string, fullConfig *toml.Tree, logger *logrus.Logger,
	eventDispatcher *events.EventDispatcher,
) {
	// Init the transport struct.
	transport.antiFloodDelay = 5
	transport.rejoinDelay = 15 * time.Second
	transport.botName = botName
	transport.user = fullConfig.GetDefault("mattermost.user", "papaBot").(string)
	transport.password = fullConfig.GetDefault("mattermost.password", "").(string)
	transport.team = fullConfig.GetDefault("mattermost.team", "team").(string)
	transport.websocket = fullConfig.GetDefault("mattermost.websocket", "ws://localhost:8065").(string)
	transport.server = fullConfig.GetDefault("mattermost.server", "localhost:8065").(string)
	transport.channels = utils.ToStringSlice(
		fullConfig.GetDefault("mattermost.channels", []string{"#papabot"}).([]interface{}))
	// State.
	transport.onChannel = map[string]*model.Channel{}
	transport.eventHandlers = map[string][]eventHandlerFunc{}
	transport.users = map[string]string{}
	// Utility objects.
	transport.log = logger
	transport.eventDispatcher = eventDispatcher
}

// Name of the transport.
func (transport *MattermostTransport) Name() string {
	return "mattermost"
}

// typingListener will pretend that the bot is typing.
func (transport *MattermostTransport) typingListener(message events.EventMessage) {
	if message.Transport == transport.Name() {
		transport.webSocketClient.UserTyping(transport.channelNameToId(message.Channel), message.Context)
	}
}

// sendEvent triggers an event for the bot.
func (transport *MattermostTransport) sendEvent(
	eventCode events.EventCode, context string, direct bool, channel, nick, userId string, message ...interface{}) {

	eventMessage := events.EventMessage{
		transport.Name(),
		eventCode,
		nick,
		userId,
		channel,
		fmt.Sprint(message...),
		context,
		direct,
	}
	transport.eventDispatcher.Trigger(eventMessage)
}

// updateInfo will update bot's information on the server, if needed.
func (transport *MattermostTransport) updateInfo() {
	if transport.mmUser.FirstName != transport.botName || transport.mmUser.LastName != "" {
		transport.mmUser.FirstName = transport.botName
		transport.mmUser.LastName = ""
		// Send update request.
		if user, response := transport.client.UpdateUser(transport.mmUser); response.Error != nil {
			transport.log.Fatal("Failed to update bot information on the server %s", response.Error)
		} else {
			transport.mmUser = user
			transport.log.Info("Bot info updated on the server.")
		}
	}
}

// updateStatus will update bot's status from the server.
func (transport *MattermostTransport) updateStatus() {
	// Get transport's team info.
	team, response := transport.client.GetTeamByName(transport.team, "")
	if response.Error != nil {
		transport.log.Fatalf("Failed to get info for team '%s' %s", transport.team, response.Error)
	}
	transport.log.Infof("I am in team '%s'.", transport.team)
	transport.mmTeam = team
}

// joinChannels will make the transport join configured channels.
func (transport *MattermostTransport) joinChannels() {
	for _, channelName := range transport.channels {
		if channel, response := transport.client.GetChannelByName(
			channelName, transport.mmTeam.Id, ""); response.Error != nil {
			transport.log.Fatalf("Failed to get info for channel '%s' %s.", channelName, response.Error)
		} else {
			transport.onChannel[channel.Id] = channel
			transport.sendEvent(
				events.EventJoinedChannel, "", true, channelName, transport.botName, transport.mmUser.Id, "")
			transport.log.Infof("Joined channel '%s'", channelName)
		}
	}
}

// imOnChannel will tell if the transport is listening on that channel.
func (transport *MattermostTransport) imOnChannel(channelId string) bool {
	if _, ok := transport.onChannel[channelId]; ok {
		return true
	}
	return false
}

// channelNameToId converts channel name to it's id.
func (transport *MattermostTransport) channelNameToId(channelName string) string {
	for id, channel := range transport.onChannel {
		if channel.Name == channelName {
			return id
		}
	}
	transport.log.Errorf("Couldn't convert channel name to id! (%s)", channelName)
	return ""
}

// openPrivateChannel will open a new channel and return the channel name.
func (transport *MattermostTransport) openPrivateChannel(nick string) (string, error) {
	userId := transport.userNickToId(nick)
	// Create a new channel.
	if newChannel, response := transport.client.CreateDirectChannel(transport.mmUser.Id, userId); response.Error != nil {
		transport.log.Errorf("Couldn't open a private channel to %s %s", nick, response.Error)
		return "", response.Error
	} else {
		transport.log.Infof("Opened a new private channel %s.", newChannel.Name)
		transport.onChannel[newChannel.Id] = newChannel
		return newChannel.Name, nil
	}
}

// userIdToNick returns the nick matching the id.
func (transport *MattermostTransport) userIdToNick(userId string) string {
	if nick, exists := transport.users[userId]; exists {
		return nick
	} else {
		if user, response := transport.client.GetUser(userId, ""); response.Error != nil {
			transport.log.Warnf("Failed to get user info for %s", userId)
			return "[unknown]"
		} else {
			transport.users[userId] = user.Username
			return user.Username
		}
	}
}

// userNickToId returns the id matching the nick.
func (transport *MattermostTransport) userNickToId(nick string) string {
	for userId, userName := range transport.users {
		if userName == nick {
			return userId
		}
	}
	// Nick not found.
	if user, response := transport.client.GetUserByUsername(nick, ""); response.Error != nil {
		transport.log.Warnf("Failed to get user info for %s", nick)
		return "[unknown]"
	} else {
		transport.users[user.Id] = user.Username
		return user.Id
	}
}

// directMessage checks whether the message was directed at the bot.
func (transport *MattermostTransport) directMessage(message string) (string, bool) {
	for _, prefix := range []string{".", transport.user, "@" + transport.user} {
		if strings.HasPrefix(message, prefix) { // Is someone talking to the bot?
			message = strings.TrimLeft(message[len(prefix):], ",:; ")
			if message != "" {
				return message, true
			}
		}
	}
	return message, false
}

// NickIsMe will do pure magic.
func (transport *MattermostTransport) NickIsMe(nick string) bool {
	return nick == transport.mmUser.Username
}

// postMessage will send a message through th eclient.
func (transport *MattermostTransport) postMessage(channel, message, context string) {
	post := &model.Post{
		ChannelId: transport.channelNameToId(channel),
		Message:   message,
		ParentId:  context,
		RootId:    context,
	}
	_, response := transport.client.CreatePost(post)
	if response.Error != nil {
		transport.log.Errorf("Failed to send message to %s %s", channel, response.Error)
	}
}

func (transport *MattermostTransport) SendMessage(sourceEvent *events.EventMessage, message string) {
	transport.postMessage(sourceEvent.Channel, message, sourceEvent.Context)
}

func (transport *MattermostTransport) SendPrivateMessage(sourceEvent *events.EventMessage, nick, message string) {
	if privChannel, err := transport.openPrivateChannel(nick); err == nil {
		transport.postMessage(privChannel, message, "")
	}
}

func (transport *MattermostTransport) SendNotice(sourceEvent *events.EventMessage, message string) {
	// There are no notices on Mattermost.
	transport.SendMessage(sourceEvent, "**"+message+"**")
}

func (transport *MattermostTransport) SendMassNotice(message string) {
	for _, channel := range transport.onChannel {
		if channel.Type != model.CHANNEL_DIRECT {  // Do not send notices to private chats.
			transport.postMessage(channel.Name, message, "")
		}
	}
}
