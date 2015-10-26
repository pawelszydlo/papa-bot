package main

import (
	"time"
	"fmt"
	"github.com/dustin/go-humanize"
)

// Check if two nicks don't belong to the same person
func AreSamePeople(nick1, nick2 string) bool {
	// TBI
	return nick1 == nick2
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
		return humanize.Time(fromTime)
	}
	return ""
}