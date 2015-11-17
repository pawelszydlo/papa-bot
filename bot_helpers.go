package papaBot

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/BurntSushi/toml"
	"golang.org/x/net/html/charset"
	"golang.org/x/text/transform"
	"html"
	"io"
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"text/template"
)

// RegisterExtension will register a new extension with the bot.
func (bot *Bot) RegisterExtension(ext extensionInterface) error {
	if ext == nil {
		return errors.New("Nil extension provided.")
	}
	bot.extensions = append(bot.extensions, ext)
	// If bot's init was already done, all other extensions have already been initialized.
	if bot.initDone {
		return ext.Init(bot)
	}
	return nil
}

// RegisterCommand will register a new command with the bot.
func (bot *Bot) RegisterCommand(cmd *BotCommand) error {
	for _, name := range cmd.CommandNames {
		for existingName, _ := range bot.commands {
			if name == existingName {
				return errors.New(fmt.Sprintf("Command under alias '%s' already exists.", name))
			}
		}
		bot.commands[name] = cmd
	}
	return nil
}

// GetChannelsOn will return a list of channels the bot is currently on.
func (bot *Bot) GetChannelsOn() []string {
	channelsOn := []string{}
	for channel, on := range bot.onChannel {
		if on {
			channelsOn = append(channelsOn, channel)
		}
	}
	return channelsOn
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
		bot.log.Debug("%s becomes %s", urlinfo.URL, final_link)
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
	if strings.Contains(contentType, "text/html") || strings.Contains(contentType, "text/plain") {
		// Try to get more significant part for encoding detection.
		webContentSampleRe := regexp.MustCompile(`(?i)<[^>]*?description[^<]*?>|<title>.*?</title>`)
		sample := bytes.Join(webContentSampleRe.FindAll(body, -1), []byte{})
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
				detectionContentType = "text/html; " + t
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
		bot.log.Debug("Not fetching the body for Content-Type: %s", contentType)
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
				bot.log.Warning("Field left empty %s!", field.Name)
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
					bot.log.Fatalf("Can't find field %s to store template from %s.", tempFieldName, field.Name)
				}
				if !tempField.CanSet() {
					bot.log.Fatalf("Field %s is not settable.", tempFieldName)
				}
				if reflect.ValueOf(temp).Type() != tempField.Type() {
					bot.log.Fatalf("Incompatible types %s and %s", reflect.ValueOf(temp).Type(), tempField.Type())
				}
				tempField.Set(reflect.ValueOf(temp))
			}
		}
	}
	if missingTexts {
		bot.log.Fatal("Missing texts.")
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
			bot.log.Error("Can't delete custom variable %s: %s", name, err)
		}
		return
	}
	bot.customVars[name] = value
	if _, err := bot.Db.Exec(`INSERT OR REPLACE INTO vars VALUES(?, ?)`, name, value); err != nil {
		bot.log.Error("Can't add custom variable %s: %s", name, err)
	}
}

// GetVar returns the value of a custom variable.
func (bot *Bot) GetVar(name string) string {
	return bot.customVars[name]
}
