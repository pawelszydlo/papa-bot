package papaBot

// Public bot API.

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/pawelszydlo/papa-bot/transports"
	"github.com/pawelszydlo/papa-bot/utils"
	"golang.org/x/net/html/charset"
	"golang.org/x/text/transform"
	"html"
	"io"
	"net/http"
	"reflect"
	"strings"
	"text/template"
	"time"
)

// RegisterExtension will register a new transport with the bot.
func (bot *Bot) RegisterTransport(name string, transport transports.Transport) {
	// Is the transport enabled in the config?
	if bot.fullConfig.GetDefault(fmt.Sprintf("%s.enabled", name), false).(bool) {
		for existingName := range bot.transports {
			if name == existingName {
				bot.Log.Fatalf("Transport under alias '%s' already exists.", name)
			}
		}
		bot.transports[name] = transport
		bot.Log.Infof("Added transport: %s", name)
	} else {
		bot.Log.Infof("Transport under alias '%s' disabled in the config.", name)
	}
}

// RegisterExtension will register a new extension with the bot.
func (bot *Bot) RegisterExtension(ext extension) {
	if ext == nil {
		bot.Log.Fatal("Nil extension provided.")
	}
	bot.extensions = append(bot.extensions, ext)
	bot.Log.Debugf("Added extension: %T", ext)
	// If bot's init was already done, all other extensions have already been initialized.
	if bot.initDone {
		if err := ext.Init(bot); err != nil {
			bot.Log.Fatalf("Error initializing extension: %s", err)
		}
	}
}

// RegisterCommand will register a new command with the bot.
func (bot *Bot) RegisterCommand(cmd *BotCommand) {
	for _, name := range cmd.CommandNames {
		for existingName := range bot.commands {
			if name == existingName {
				bot.Log.Fatalf("Command under alias '%s' already exists.", name)
			}
		}
		bot.commands[name] = cmd
	}
}

// SendMessage sends a message to the channel.
func (bot *Bot) SendMessage(transportName, channel, message string) {
	bot.Log.Debugf("Sending message to [%s]%s: %s", transportName, channel, message)
	transport := bot.getTransportOrDie(transportName)
	transport.SendMessage(channel, message)
}

// SendPrivMessage sends a private message to a user.
func (bot *Bot) SendPrivMessage(transportName, nick, message string) {
	bot.Log.Debugf("Sending private message to [%s]%s: %s", transportName, nick, message)
	transport := bot.getTransportOrDie(transportName)
	transport.SendPrivMessage(nick, message)
}

// SendAutoMessage is a helper function for sending either a public message to channel or private to nick.
func (bot *Bot) SendAutoMessage(private bool, transportName, nick, channel, message string) {
	if private {
		bot.SendPrivMessage(transportName, nick, message)
	} else {
		bot.SendMessage(transportName, channel, message)
	}
}

// SendNotice sends a notice to the channel.
func (bot *Bot) SendNotice(transportName, channel, message string) {
	bot.Log.Debugf("Sending notice to %s: [%s]%s", transportName, channel, message)
	transport := bot.getTransportOrDie(transportName)
	transport.SendNotice(channel, message)
}

// SendPrivNotice sends a private notice to a user.
func (bot *Bot) SendPrivNotice(transportName, nick, message string) {
	bot.Log.Debugf("Sending private notice to [%s]%s: %s", transportName, nick, message)
	transport := bot.getTransportOrDie(transportName)
	transport.SendPrivNotice(nick, message)
}

// SendAutoNotice is a helper function for sending either a public notice to channel or private to nick.
func (bot *Bot) SendAutoNotice(private bool, transportName, nick, channel, message string) {
	if private {
		bot.SendPrivNotice(transportName, nick, message)
	} else {
		bot.SendNotice(transportName, channel, message)
	}
}

// SendMassNotice sends a notice to all the channels bot is on, on all transports.
func (bot *Bot) SendMassNotice(message string) {
	bot.Log.Debugf("Sending mass notice: %s", message)
	for _, transport := range bot.transports {
		transport.SendMassNotice(message)
	}
}

// GetPageBody gets and returns a body of a page. Return format is error, final url, body.
func (bot *Bot) GetPageBody(URL string, customHeaders map[string]string) (error, string, []byte) {
	if URL == "" {
		return errors.New("Empty URL"), "", nil
	}
	// Build the request.
	req, err := http.NewRequest("GET", URL, nil)
	if err != nil {
		return err, "", nil
	}
	if customHeaders == nil {
		customHeaders = map[string]string{}
	}
	if customHeaders["User-Agent"] == "" {
		customHeaders["User-Agent"] = bot.Config.HttpDefaultUserAgent
	}
	for k, v := range customHeaders {
		req.Header.Set(k, v)
	}

	// Get response.
	resp, err := bot.HTTPClient.Do(req)
	if err != nil {
		return err, "", nil
	}
	defer resp.Body.Close()

	// Update the URL if it changed after redirects.
	finalLink := resp.Request.URL.String()
	if finalLink != "" && finalLink != URL {
		bot.Log.Debugf("%s becomes %s", URL, finalLink)
		URL = finalLink
	}

	// Load the body up to PageBodyMaxSize.
	body := make([]byte, bot.Config.PageBodyMaxSize, bot.Config.PageBodyMaxSize)
	if num, err := io.ReadFull(resp.Body, body); err != nil && err != io.ErrUnexpectedEOF {
		return err, finalLink, nil
	} else {
		// Trim unneeded 0 bytes so that JSON unmarshaller won't complain.
		body = body[:num]
	}
	// Get the content-type
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(body)
	}

	// If type is text, decode the body to UTF-8.
	if strings.Contains(contentType, "text/") {
		// Try to get more significant part for encoding detection.
		sample := bytes.Join(bot.webContentSampleRe.FindAll(body, -1), []byte{})
		if len(sample) < 100 {
			sample = body
		}
		// Unescape HTML tokens.
		sample = []byte(html.UnescapeString(string(sample)))
		// Try to only get charset from content type. Needed because some pages serve broken Content-Type header.
		detectionContentType := contentType
		tokens := strings.Split(contentType, ";")
		for _, t := range tokens {
			if strings.Contains(strings.ToLower(t), "charset") {
				detectionContentType = "text/plain; " + t
				break
			}
		}
		// Detect encoding and transform.
		encoding, _, _ := charset.DetermineEncoding(sample, detectionContentType)
		decodedBody, _, _ := transform.Bytes(encoding.NewDecoder(), body)
		return nil, finalLink, decodedBody
	} else if strings.Contains(contentType, "application/json") {
		return nil, finalLink, body
	} else {
		bot.Log.Debugf("Not fetching the body for Content-Type: %s", contentType)
	}
	return nil, "", nil
}

// LoadTexts loads texts from a section of a config file into a struct, auto handling templates and lists.
// The name of the field in the data struct defines the name in the config file.
// The type of the field determines the expected config value.
func (bot *Bot) LoadTexts(section string, data interface{}) error {

	reflectedData := reflect.ValueOf(data).Elem()

	for i := 0; i < reflectedData.NumField(); i++ {
		fieldDef := reflectedData.Type().Field(i)
		// Get the field name.
		fieldName := fieldDef.Name
		// Get the field type name.
		fieldType := fmt.Sprint(fieldDef.Type)
		// Get the field itself.
		field := reflectedData.FieldByName(fieldName)
		if !field.CanSet() {
			bot.Log.Fatalf("Field %s is not settable.", fieldName)
		}

		// Load configured text for the field.
		key := fmt.Sprintf("%s.%s", section, fieldName)
		if !bot.fullTexts.Has(key) {
			bot.Log.Fatalf("Couldn't load text for field %s, key %s.", fieldName, key)
		}

		if fieldType == "*template.Template" { // This field is a template.
			temp, err := template.New(fieldName).Parse(bot.fullTexts.Get(key).(string))
			if err != nil {
				return err
			} else {
				field.Set(reflect.ValueOf(temp))
			}
		} else if fieldType == "string" { // Regular text field.
			field.Set(reflect.ValueOf(bot.fullTexts.Get(key).(string)))
		} else if fieldType == "[]string" {
			field.Set(reflect.ValueOf(utils.ToStringSlice(bot.fullTexts.Get(key).([]interface{}))))
		} else {
			bot.Log.Fatalf("Unsupported type of text field: %s", fieldType)
		}
	}

	return nil
}

// SetVar will set a custom variable. Set to empty string to delete.
func (bot *Bot) SetVar(name, value string) {
	if name == "" {
		return
	}
	// Delete.
	if value == "" {
		delete(bot.customVars, name)
		if _, err := bot.Db.Exec(`DELETE FROM vars WHERE name=?`, name); err != nil {
			bot.Log.Errorf("Can't delete custom variable %s: %s", name, err)
		}
		return
	}
	bot.customVars[name] = value
	if _, err := bot.Db.Exec(`INSERT OR REPLACE INTO vars VALUES(?, ?)`, name, value); err != nil {
		bot.Log.Errorf("Can't add custom variable %s: %s", name, err)
	}
}

// GetVar returns the value of a custom variable.
func (bot *Bot) GetVar(name string) string {
	return bot.customVars[name]
}

// IsOnChannel checks whether the bot is present on a given channel.
func (bot *Bot) IsOnChannel(transportName, name string) bool {
	transport := bot.getTransportOrDie(transportName)
	if val, ok := transport.OnChannels()[name]; ok && val {
		return true
	}
	return false
}

// AddMoreInfo will set more information to be viewed for the channel.
func (bot *Bot) AddMoreInfo(transport, channel, info string) error {
	if !bot.IsOnChannel(transport, channel) {
		return errors.New("I'm not on channel " + channel)
	}
	bot.urlMoreInfo[transport+channel] = info
	return nil
}

// NextDailyTick will get the time for bot's next daily tick.
func (bot *Bot) NextDailyTick() time.Time {
	tick := bot.nextDailyTick
	return tick
}

// AddToIgnoreList will add a user to the ignore list.
func (bot *Bot) AddToIgnoreList(fullName string) {
	ignored := strings.Split(bot.GetVar("_ignored"), " ")
	ignored = utils.RemoveDuplicates(append(ignored, fullName))
	bot.SetVar("_ignored", strings.Join(ignored, " "))
	// Update the actual blocklist in the event handler.
	bot.EventDispatcher.SetBlackList(ignored)
	bot.Log.Infof("%s added to ignore list.", fullName)
}

// RemoveFromIgnoreList will remove user from the ignore list.
func (bot *Bot) RemoveFromIgnoreList(fullName string) {
	ignored := strings.Split(bot.GetVar("_ignored"), " ")
	ignored = utils.RemoveFromSlice(ignored, fullName)
	bot.SetVar("_ignored", strings.Join(ignored, " "))
	// Update the actual blocklist in the event handler.
	bot.EventDispatcher.SetBlackList(ignored)
	bot.Log.Infof("%s removed from ignore list.", fullName)
}
