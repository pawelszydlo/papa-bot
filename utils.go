package papaBot

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"golang.org/x/crypto/pbkdf2"
	"log"
	"os"
	"sort"
	"text/template"
	"time"
)

// This part comes from https://github.com/dustin/go-humanize, copied for localization
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
	{Hour, "%d minut temu", Minute},
	{2 * Hour, "1 godzinę temu", 1},
	{Day, "%d godzin temu", Hour},
	{2 * Day, "1 dzień temu", 1},
	{Week, "%d dni temu", Day},
	{2 * Week, "1 tydzień temu", 1},
	{Month, "%d tygodni temu", Week},
	{2 * Month, "1 miesiąc temu", 1},
	{Year, "%d miesięcy temu", Month},
	{18 * Month, "1 rok temu", 1},
	{2 * Year, "2 lata temu", 1},
	{LongTime, "%d lat temu", Year},
}

// HashPassword hashes the password.
func HashPassword(password string) string {
	return fmt.Sprintf("hash:%s", base64.StdEncoding.EncodeToString(
		pbkdf2.Key([]byte(password), []byte(password), 4096, sha256.Size, sha256.New)))
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

// TimeDiff returns a humanized time passed string.
func TimePassed(past time.Time) string {
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

// GetTimeElapsed adds current timezone to passed fromTime and gets string describing time elapsed.
func GetTimeElapsed(fromTime time.Time) string {
	// Hack to force the time to be from the same timezone as now
	fromTime, err := time.ParseInLocation(
		"2006-01-02 15:04:05",
		fmt.Sprintf("%d-%02d-%02d %02d:%02d:%02d",
			fromTime.Year(), fromTime.Month(), fromTime.Day(),
			fromTime.Hour(), fromTime.Minute(), fromTime.Second()),
		time.Now().Location())
	if err != nil {
		log.Fatal("Date parse error:", err)
	}

	return TimePassed(fromTime)
}

// Format formats the template with passed string map values.
func Format(tpl *template.Template, params *map[string]string) string {
	var text bytes.Buffer
	if err := tpl.Execute(&text, params); err != nil {
		log.Fatalln("Error executing template", err)
		return ""
	}
	return text.String()
}
