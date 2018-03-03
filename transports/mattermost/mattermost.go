package mattermostTransport

import (
	"github.com/mattermost/mattermost-server/model"
	"github.com/pawelszydlo/papa-bot/events"
)

// connect will establish a connection to the server.
func (transport *MattermostTransport) connect() {
	// Create the client.
	transport.client = model.NewAPIv4Client(transport.server)

	// Check server connection
	if props, response := transport.client.GetOldClientConfig(""); response.Error != nil {
		transport.log.Fatalf("Couldn't connect to Mattermost at %s.", transport.server)
	} else {
		transport.log.Infof("Connected to %s, running version %s.", transport.server, props["Version"])
	}

	// Login.
	if user, response := transport.client.Login(transport.user, transport.password); response.Error != nil {
		transport.log.Fatalf("Login failed as %s.", transport.user)
	} else {
		transport.mmUser = user
		transport.log.Infof("Logged in as %s (%s).", user.Username, user.Id)
	}
	transport.sendEvent(events.EventConnected, true, "", transport.botName, transport.mmUser.Id, "")
}

// Run will execute the main loop.
func (transport *MattermostTransport) Run() {
	transport.connect()
	transport.updateInfo()
	transport.updateStatus()

	// Register event handlers.
	transport.registerAllEventHandlers()

	// Start websocket for communication.
	webSocketClient, err := model.NewWebSocketClient4(transport.websocket, transport.client.AuthToken)
	if err != nil {
		transport.log.Fatalf("Failed to connect to the web socket at %s: %s", transport.websocket, err)
	}
	webSocketClient.Listen()

	// Main loop.
	for {
		select {
		case event, ok := <-webSocketClient.EventChannel:
			if ok {
				// Are there any handlers registered for this event?
				if handlers, exists := transport.eventHandlers[event.Event]; exists {
					for _, handler := range handlers {
						handler(event)
					}
				} else { // No handler for this type of event.
					// transport.log.Debugf("No handler for event: %s", event.Event)
				}
			} else {
				break
			}
		}
	}

	transport.log.Infof("Mattermost transport exiting...")
}
