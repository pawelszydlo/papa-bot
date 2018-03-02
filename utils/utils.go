// Helpful utility functions.
package utils

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"golang.org/x/crypto/pbkdf2"
	"golang.org/x/net/idna"
	"html"
	"log"
	"os"
	"sort"
	"strings"
	"text/template"
	"time"
	"unicode"
)

// TODO: refactor date humanization to enable l18n.
const (
	Minute   = 60
	Hour     = 60 * Minute
	Day      = 24 * Hour
	Week     = 7 * Day
	Month    = 30 * Day
	Year     = 12 * Month
	LongTime = 37 * Year
)

var magnitudes = []struct {
	diff   int64
	format string
	divby  int64
}{
	{60, "chwilkę temu", 1},
	{2 * Minute, "1 minutę temu", 1},
	{5 * Minute, "%d minuty temu", Minute},
	{Hour, "%d minut temu", Minute},
	{2 * Hour, "1 godzinę temu", 1},
	{5 * Hour, "%d godziny temu", Hour},
	{Day, "%d godzin temu", Hour},
	{2 * Day, "1 dzień temu", 1},
	{Week, "%d dni temu", Day},
	{2 * Week, "1 tydzień temu", 1},
	{5 * Week, "%d tygodnie temu", Week},
	{Month, "%d tygodni temu", Week},
	{2 * Month, "1 miesiąc temu", 1},
	{5 * Month, "%d miesiące temu", Month},
	{Year, "%d miesięcy temu", Month},
	{18 * Month, "1 rok temu", 1},
	{5 * Year, "%d lata temu", Year},
	{LongTime, "%d lat temu", Year},
}

// HumanizedSince returns a humanized time passed string.
func HumanizedSince(past time.Time) string {
	diff := time.Now().Unix() - past.Unix()

	// Find the magnitude closest but bigger then diff.
	n := sort.Search(len(magnitudes), func(i int) bool {
		return magnitudes[i].diff > diff
	})

	magnitude := magnitudes[n]
	// If magnitude has a placeholder for a number, insert it.
	if strings.Contains(magnitude.format, "%d") {
		return fmt.Sprintf(magnitude.format, diff/magnitude.divby)
	}
	return magnitude.format
}

// StripTags strips HTML tags from text.
func StripTags(text string) string {
	output := bytes.NewBufferString("")
	inTag := false
	for _, r := range text {
		switch r {
		case '<':
			inTag = true
		case '>':
			inTag = false
		default:
			if !inTag {
				output.WriteRune(r)
			}
		}
	}
	return output.String()
}

// MustForceLocalTimezone adds current timezone to passed date, without recalculating the date.
func MustForceLocalTimezone(date time.Time) time.Time {
	// Hack to force the time to be from the same timezone as now
	date, err := time.ParseInLocation(
		"2006-01-02 15:04:05",
		fmt.Sprintf("%d-%02d-%02d %02d:%02d:%02d",
			date.Year(), date.Month(), date.Day(),
			date.Hour(), date.Minute(), date.Second()),
		time.Now().Location())
	if err != nil {
		log.Fatal("Date parse error:", err)
	}

	return date
}

// DirExists returns whether the given file or directory exists or not.
func DirExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return true, err
}

// Format formats the template with passed string map values.
func Format(tpl *template.Template, params map[string]string) string {
	var text bytes.Buffer
	if err := tpl.Execute(&text, params); err != nil {
		log.Fatalln("Error executing template", err)
		return ""
	}
	return text.String()
}

// CleanString cleans a string from new lines and caret returns, un-escapes HTML entities and trims spaces.
func CleanString(str string, unescape bool) string {
	if unescape {
		str = html.UnescapeString(str)
	}
	str = strings.Replace(str, "\n", "", -1)
	str = strings.Replace(str, "\r", "", -1)
	return strings.TrimFunc(str, unicode.IsSpace)
}

// StandardizeURL standardizes the url by making sure it has a schema and converting IDNA domains into ASCII.
func StandardizeURL(url string) string {
	link := url
	var schema, domain, path string

	// Try to get the schema
	slice := strings.SplitN(url, "://", 2)
	if len(slice) == 2 && len(slice[0]) < 10 { // schema exists
		schema = slice[0] + "://"
		link = slice[1]
	} else {
		schema = "http://"
	}

	// Get the domain
	slice = strings.SplitN(link, "/", 2)
	if len(slice) == 2 {
		domain = slice[0]
		path = "/" + slice[1]
	} else {
		domain = slice[0]
		path = "/"
	}

	domain, _ = idna.ToASCII(domain)
	link = schema + domain + path

	return link
}

// SliceToMap will convert a string slice into a string map.
func SliceToMap(slice []string) map[string]bool {
	unique := map[string]bool{}
	for _, elem := range slice {
		unique[elem] = true
	}
	return unique
}

// MapToSlice will save keys of a string map into a slice.
func MapToSlice(boolMap map[string]bool) []string {
	slice := []string{}
	for key := range boolMap {
		slice = append(slice, key)
	}
	return slice
}

// RemoveDuplicates will remove duplicates from a slice of strings.
func RemoveDuplicates(slice []string) []string {
	return MapToSlice(SliceToMap(slice))
}

// RemoveFromSlice will remove an element from string slice by value, assuming the element is there only once.
func RemoveFromSlice(slice []string, item string) []string {
	for i, value := range slice {
		if value == item {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}

// HashPassword hashes a password.
func HashPassword(password string) string {
	return base64.StdEncoding.EncodeToString(
		pbkdf2.Key([]byte(password), []byte(password), 4096, sha256.Size, sha256.New))
}

// ToStringSlice converts arbitrary interface slice into a string slice.
func ToStringSlice(elements []interface{}) []string {
	strs := make([]string, len(elements), len(elements))
	for i := range elements {
		strs[i] = elements[i].(string)
	}
	return strs
}
