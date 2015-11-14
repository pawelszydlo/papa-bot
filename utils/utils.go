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
	"strings"
	"text/template"
	"time"
	"unicode"
)

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

// SliceToMap will convert a tring slice into a string map.
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
	for key, _ := range boolMap {
		slice = append(slice, key)
	}
	return slice
}

// RemoveDuplicates will remove duplicates from a slice of strings.
func RemoveDuplicates(slice []string) []string {
	return MapToSlice(SliceToMap(slice))
}

// HashPassword hashes a password.
func HashPassword(password string) string {
	return base64.StdEncoding.EncodeToString(
		pbkdf2.Key([]byte(password), []byte(password), 4096, sha256.Size, sha256.New))
}
