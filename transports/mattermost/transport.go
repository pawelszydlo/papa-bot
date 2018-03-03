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

	// Transport name.
	transportName string
	// Mattermost client
	client *model.Client4
	// Mattermost identity.
	mmUser *model.User
	mmTeam *model.Team
	// Anti flood buffered semaphore
	floodSemaphore chan int
	// Channels the bot is on. channelId -> name
	onChannel map[string]string
	// Registered event handlers.
	eventHandlers map[string][]eventHandlerFunc
	// User identification cache userId -> nick
	users map[string]string
}

// Init initializes a transport instance.
func (transport *MattermostTransport) Init(transportName, botName string, fullConfig *toml.Tree, logger *logrus.Logger,
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
	transport.floodSemaphore = make(chan int, 5)
	transport.onChannel = map[string]string{}
	transport.eventHandlers = map[string][]eventHandlerFunc{}
	transport.users = map[string]string{}
	// Utility objects.
	transport.log = logger
	transport.eventDispatcher = eventDispatcher
	transport.transportName = transportName
}

// sendEvent triggers an event for the bot.
func (transport *MattermostTransport) sendEvent(eventCode events.EventCode, direct bool, channel, nick, userId string, message ...interface{}) {
	eventMessage := events.EventMessage{
		transport.transportName,
		eventCode,
		nick,
		userId,
		channel,
		fmt.Sprint(message...),
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
		if channel, response := transport.client.GetChannelByName(channelName, transport.mmTeam.Id, ""); response.Error != nil {
			transport.log.Fatalf("Failed to get info for channel '%s' %s.", channelName, response.Error)
		} else {
			transport.onChannel[channel.Id] = channelName
			transport.sendEvent(events.EventJoinedChannel, true, channelName, transport.botName, transport.mmUser.Id, "")
			transport.log.Infof("Joined channel '%s'", channelName)
		}
	}
}

// OnChannels returns all channels the bot is on.
func (transport *MattermostTransport) OnChannels() map[string]bool {
	chanMap := make(map[string]bool)
	for _, chanName := range transport.onChannel {
		chanMap[chanName] = true
	}
	return chanMap
}

// imOnChannel will tell if the transport is listening on that channel.
func (transport *MattermostTransport) imOnChannel(channelId string) bool {
	if _, ok := transport.onChannel[channelId]; ok {
		return true
	}
	return false
}

// channelNameToId converts channel name to it's id.
func (transport *MattermostTransport) channelNameToId(channel string) string {
	for id, name := range transport.onChannel {
		if name == channel {
			return id
		}
	}
	transport.log.Errorf("Couldn't convert channel name to id! (%s)", channel)
	return ""
}

// openPrivateChannel will open a new channel, invite the nick and return the channel name.
func (transport *MattermostTransport) openPrivateChannel(nick string) (string, error) {
	userId := transport.userNickToId(nick)
	channelName := transport.mmUser.Id + "_" + userId
	// Make sure the transport is not on the channel yet.
	channelId := transport.channelNameToId(channelName)
	if channelId != "" {
		return channelName, nil
	}
	// Create a new channel.
	if newChannel, response := transport.client.CreateDirectChannel(transport.mmUser.Id, userId); response.Error != nil {
		transport.log.Errorf("Couldn't open a private channel to %s %s", nick, response.Error)
		return "", response.Error
	} else {
		transport.log.Infof("Opened a new private channel %s.", newChannel.Name)
		transport.onChannel[newChannel.Id] = newChannel.Name
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

func (transport *MattermostTransport) SendMessage(channel, message string) {
	post := &model.Post{
		ChannelId: transport.channelNameToId(channel),
		Message:   message,
	}
	_, response := transport.client.CreatePost(post)
	if response.Error != nil {
		transport.log.Errorf("Failed to send message to %s %s", channel, response.Error)
	}
}

func (transport *MattermostTransport) SendPrivMessage(user, message string) {
	if privChannel, err := transport.openPrivateChannel(user); err == nil {
		transport.SendMessage(privChannel, message)
	}
}

func (transport *MattermostTransport) SendNotice(channel, message string) {
	// There are no notices on Mattermost.
	transport.SendMessage(channel, "> "+message)
}

func (transport *MattermostTransport) SendPrivNotice(user, message string) {
	// There are no notices on Mattermost.
	if privChannel, err := transport.openPrivateChannel(user); err == nil {
		transport.SendMessage(privChannel, "> "+message)
	}
}

func (transport *MattermostTransport) SendMassNotice(message string) {
	for _, channel := range transport.onChannel {
		transport.SendNotice(channel, message)
	}
}
