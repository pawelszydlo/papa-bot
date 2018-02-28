package papaBot

// Public bot API.

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/pawelszydlo/papa-bot/transports"
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
func (bot *Bot) RegisterTransport(name string, newFunction newTransportFunction) {
	// Is the transport enabled in the config?
	if bot.fullConfig.GetDefault(fmt.Sprintf("%s.enabled", name), false).(bool) {
		for existingName := range bot.transports {
			if name == existingName {
				bot.Log.Fatalf("Transport under alias '%s' already exists.", name)
			}
		}
		bot.Log.Infof("Registering transport: %s...", name)
		scribeChannel := make(chan transports.ScribeMessage, MessageBufferSize)
		commandChannel := make(chan transports.CommandMessage, MessageBufferSize)
		bot.transports[name] = transportWrapper{
			newFunction(bot.Config.Name, bot.fullConfig, bot.Log, scribeChannel, commandChannel),
			scribeChannel,
			commandChannel,
		}
		bot.Log.Debugf("Added extension: %s", name)
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
func (bot *Bot) SendPrivMessage(transport, channel, message string) {
	bot.Log.Debugf("Sending message to %s-%s: %s", transport, channel, message)
	wrap := bot.getTransportWrapOrDie(transport)
	wrap.transport.SendPrivMessage(channel, message)
}

// SendNotice sends a notice to the channel.
func (bot *Bot) SendNotice(transport, channel, message string) {
	bot.Log.Debugf("Sending notice to %s-%s: %s", transport, channel, message)
	wrap := bot.getTransportWrapOrDie(transport)
	wrap.transport.SendNotice(channel, message)
}

// SendMassNotice sends a notice to all the channels bot is on, on all transports.
func (bot *Bot) SendMassNotice(message string) {
	bot.Log.Debugf("Sending mass notice: %s", message)
	for _, wrap := range bot.transports {
		wrap.transport.SendMassNotice(message)
	}
}

// GetPageBodyByURL is a convenience wrapper around GetPageBody.
func (bot *Bot) GetPageBodyByURL(url string) ([]byte, error) {
	var urlinfo UrlInfo
	urlinfo.URL = url
	if err := bot.GetPageBody(&urlinfo, map[string]string{}); err != nil {
		return urlinfo.Body, err
	}
	return urlinfo.Body, nil
}

// GetPageBody gets and returns a body of a page.
func (bot *Bot) GetPageBody(urlinfo *UrlInfo, customHeaders map[string]string) error {
	if urlinfo.URL == "" {
		return errors.New("Empty URL")
	}
	// Build the request.
	req, err := http.NewRequest("GET", urlinfo.URL, nil)
	if err != nil {
		return err
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
		return err
	}
	defer resp.Body.Close()

	// Update the URL if it changed after redirects.
	final_link := resp.Request.URL.String()
	if final_link != "" && final_link != urlinfo.URL {
		bot.Log.Debugf("%s becomes %s", urlinfo.URL, final_link)
		urlinfo.URL = final_link
	}

	// Load the body up to PageBodyMaxSize.
	body := make([]byte, bot.Config.PageBodyMaxSize, bot.Config.PageBodyMaxSize)
	if num, err := io.ReadFull(resp.Body, body); err != nil && err != io.ErrUnexpectedEOF {
		return err
	} else {
		// Trim unneeded 0 bytes so that JSON unmarshaller won't complain.
		body = body[:num]
	}
	// Get the content-type
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(body)
	}
	urlinfo.ContentType = contentType

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
		urlinfo.Body = decodedBody
	} else if strings.Contains(contentType, "application/json") {
		urlinfo.Body = body
	} else {
		bot.Log.Debugf("Not fetching the body for Content-Type: %s", contentType)
	}
	return nil
}

// LoadTexts loads texts from a file into a struct, auto handling the templates.
func (bot *Bot) LoadTexts(filename string, data interface{}) error {

	// Decode TOML
	if _, err := toml.DecodeFile(filename, data); err != nil {
		return err
	}

	// Fields starting with "Tpl" with be parsed into templates and saved in the field starting with "Temp".
	rData := reflect.ValueOf(data).Elem()
	missingTexts := false
	for i := 0; i < rData.NumField(); i++ {
		// Get field and it's value.
		field := rData.Type().Field(i)
		fieldValue := rData.Field(i)

		// Check if all fields were filled.
		if !strings.HasPrefix(field.Name, "Temp") {
			if fieldValue.String() == "" {
				bot.Log.Warningf("Field left empty %s!", field.Name)
				missingTexts = true
			}
		}

		if strings.HasPrefix(field.Name, "Tpl") {
			temp, err := template.New(field.Name).Parse(fieldValue.String())
			if err != nil {
				return err
			} else {
				tempFieldName := strings.TrimPrefix(field.Name, "Tpl")
				tempFieldName = "Temp" + tempFieldName
				// Set template field value.
				tempField := rData.FieldByName(tempFieldName)
				if !tempField.IsValid() {
					bot.Log.Fatalf("Can't find field %s to store template from %s.", tempFieldName, field.Name)
				}
				if !tempField.CanSet() {
					bot.Log.Fatalf("Field %s is not settable.", tempFieldName)
				}
				if reflect.ValueOf(temp).Type() != tempField.Type() {
					bot.Log.Fatalf("Incompatible types %s and %s", reflect.ValueOf(temp).Type(), tempField.Type())
				}
				tempField.Set(reflect.ValueOf(temp))
			}
		}
	}
	if missingTexts {
		bot.Log.Fatal("Missing texts.")
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
func (bot *Bot) IsOnChannel(transport, name string) bool {
	wrap := bot.getTransportWrapOrDie(transport)
	if val, ok := wrap.transport.OnChannels()[name]; ok && val {
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
