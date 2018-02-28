package ircTransport

import (
	"crypto/tls"
	"github.com/Sirupsen/logrus"
	"github.com/pawelszydlo/papa-bot/transports"
	"github.com/pawelszydlo/papa-bot/utils"
	"github.com/pelletier/go-toml"
	"github.com/sorcix/irc"
	"net"
	"time"
)

// New creates a new transport instance.
func New(name string, fullConfig *toml.Tree, logger *logrus.Logger,
	scribeChannel chan transports.ScribeMessage,
	commandChannel chan transports.CommandMessage,
) transports.Transport {

	// Init the transport.
	transport := &IRCTransport{
		messages: make(chan *irc.Message),

		antiFloodDelay: 5,
		rejoinDelay:    15 * time.Second,
		name:           name,
		user:           fullConfig.GetDefault("irc.user", "papaBot").(string),
		server:         fullConfig.GetDefault("irc.server", "localhost:6667").(string),
		channels:       utils.ToStringSlice(fullConfig.GetDefault("irc.channels", []string{"#papabot"}).([]interface{})),

		floodSemaphore:   make(chan int, 5),
		kickedFrom:       map[string]bool{},
		onChannel:        map[string]bool{},
		ircEventHandlers: make(map[string][]ircEvenHandlerFunc),

		log:            logger,
		scribeChannel:  scribeChannel,
		commandChannel: commandChannel,
	}

	// Attach event handlers.
	transport.assignEventHandlers()

	return transport
}

// connect attempts to connect to the given IRC server.
func (transport *IRCTransport) connect() error {
	var conn net.Conn
	var err error
	// Establish the connection.
	if transport.tlsConfig == nil {
		transport.log.Infof("Connecting to %s...", transport.server)
		conn, err = net.Dial("tcp", transport.server)
	} else {
		transport.log.Infof("Connecting to %s using TLS...", transport.server)
		conn, err = tls.Dial("tcp", transport.server, transport.tlsConfig)
	}
	if err != nil {
		return err
	}

	// Store connection.
	transport.connection = conn
	transport.decoder = irc.NewDecoder(conn)
	transport.encoder = irc.NewEncoder(conn)

	// Send initial messages.
	if transport.password != "" {
		transport.SendRawMessage(irc.PASS, []string{transport.password}, "")
	}
	transport.SendRawMessage(irc.NICK, []string{transport.name}, "")
	transport.SendRawMessage(irc.USER, []string{transport.user, "0", "*"}, transport.user)

	transport.log.Debugf("Succesfully connected.")
	return nil
}

// receiverLoop attempts to read from the IRC server and keep the connection open.
func (transport *IRCTransport) receiverLoop() {
	for {
		transport.connection.SetDeadline(time.Now().Add(300 * time.Second))
		msg, err := transport.decoder.Decode()
		if err != nil { // Error or timeout.
			transport.log.Warningf("Disconnected from server.")
			transport.connection.Close()
			retries := 0
			for {
				time.Sleep(time.Duration(retries*retries) * time.Second)
				transport.log.Infof("Reconnecting...")
				if err := transport.connect(); err == nil {
					break
				}
				retries += 1
			}
		} else {
			transport.messages <- msg
		}
	}
}

// resetFloodSemaphore flushes transport's flood semaphore.
func (transport *IRCTransport) resetFloodSemaphore() {
	for {
		select {
		case <-transport.floodSemaphore:
			continue
		default:
			return
		}
	}
}

// cleanUp cleans up after the transport.
func (transport *IRCTransport) cleanUp() {
	close(transport.messages)
	close(transport.floodSemaphore)
}

// Run starts the transport's main loop.
func (transport *IRCTransport) Run() {
	defer transport.cleanUp()

	// Connect to server.
	if err := transport.connect(); err != nil {
		transport.log.Fatalf("Error creating connection: ", err)
	}

	// Receiver loop.
	go transport.receiverLoop()

	// Semaphore clearing ticker.
	ticker := time.NewTicker(time.Second * time.Duration(transport.antiFloodDelay))
	defer ticker.Stop()
	go func() {
		for range ticker.C {
			transport.resetFloodSemaphore()
		}
	}()

	// Main loop.
	for {
		select {
		case msg, ok := <-transport.messages:
			if ok {
				// Are there any handlers registered for this IRC event?
				if handlers, exists := transport.ircEventHandlers[msg.Command]; exists {
					for _, handler := range handlers {
						handler(transport, msg)
					}
				}
			} else {
				break
			}
		}
	}

	transport.log.Infof("IRC transport exiting...")
}
