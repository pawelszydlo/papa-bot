package papaBot

import (
	"bytes"
	"fmt"
	"golang.org/x/net/idna"
	"html"
	"log"
	"os"
	"sort"
	"strings"
	"text/template"
	"time"
)

// TODO: refactor humanization.
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
	args := []interface{}{}
	escaped := false
	for _, char := range magnitude.format {
		if escaped {
			switch char {
			case '%':
			case 'd':
				args = append(args, diff/magnitude.divby)
			}
			escaped = false
		} else {
			escaped = char == '%'
		}
	}
	return fmt.Sprintf(magnitude.format, args...)
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
func CleanString(str string) string {
	str = html.UnescapeString(str)
	str = strings.Replace(str, "\n", "", -1)
	str = strings.Replace(str, "\r", "", -1)
	return strings.Trim(str, " ")
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
