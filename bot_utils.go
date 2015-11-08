package papaBot

// Various utility functions.

import (
	"fmt"
	"sort"
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
