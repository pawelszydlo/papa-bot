package main

import (
	"time"
	"fmt"
	"sort"
)

const (
	Minute   = 60
	Hour     = 60 * Minute
	Day      = 24 * Hour
	Week     = 7 * Day
	Month    = 30 * Day
	Year     = 12 * Month
	LongTime = 37 * Year
)


// Check if the sender is the bot
func isMe(name string) bool {
	return name == bot.OriginalName
}

// Check if two nicks don't belong to the same person
func AreSamePeople(nick1, nick2 string) bool {
	// TBI
	return nick1 == nick2
}

// This part comes from https://github.com/dustin/go-humanize
func TimeDiff(a time.Time) string {
	b := time.Now()
	lbl := txtAgo
	diff := b.Unix() - a.Unix()

	after := a.After(b)
	if after {
		lbl = txtFromNow
		diff = a.Unix() - b.Unix()
	}

	n := sort.Search(len(magnitudes), func(i int) bool {
		return magnitudes[i].d > diff
	})

	mag := magnitudes[n]
	args := []interface{}{}
	escaped := false
	for _, ch := range mag.format {
		if escaped {
			switch ch {
			case '%':
			case 's':
				args = append(args, lbl)
			case 'd':
				args = append(args, diff/mag.divby)
			}
			escaped = false
		} else {
			escaped = ch == '%'
		}
	}
	return fmt.Sprintf(mag.format, args...)
}


// Get string describing time elapsed
func GetTimeElapsed(fromTime time.Time) string {
	// Hack to force the time to be from the same timezone as now
	fromTime, err := time.ParseInLocation(
		"2006-01-02 15:04:05",
		fmt.Sprintf("%d-%02d-%02d %02d:%02d:%02d",
			fromTime.Year(), fromTime.Month(), fromTime.Day(),
			fromTime.Hour(), fromTime.Minute(), fromTime.Second()),
		time.Now().Location())
	if err != nil {
		lerror.Fatal("Date parse error:", err)
	}


	diff := time.Now().Unix() - fromTime.Unix()
	if diff < 60 {
		return txtJustNow
	} else  {
		return TimeDiff(fromTime)
	}
	return ""
}
