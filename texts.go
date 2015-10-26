package main

import (
	"fmt"
	"math"
)

const (
	txtDuplicateYou string = "Sam żeś"
	txtDuplicateFirst string = "%s już to wrzucał, %s."
	txtDuplicateMulti string = "Było już wrzucane %d razy. %s ostatnio wrzucał, %s."
	txtJustNow string = "chwilkę temu"
	txtNeedsPriv string = "Takie rozmowy to na priv."
	txtPasswordOk string = "Tak, panie."
	txtAgo string = "temu"
	txtFromNow string = "w przód"
)

var (
	txtHellos = []string{"Czołem lewary!", "No cześć.", "Niech będzie pochwalony Jezus Chrystus!", "Szczęść Boże!"}
	txtHellosAfterKick = []string {"Wróciłem!", "Ładnie to tak?", "Mnie? Bohatera?", "Wybaczam."}
)

// This part comes from https://github.com/dustin/go-humanize, copied for localization
var magnitudes = []struct {
	d      int64
	format string
	divby  int64
}{
	{1, "teraz", 1},
	{2, "1 sekundę %s", 1},
	{Minute, "%d sekund %s", 1},
	{2 * Minute, "1 minutę %s", 1},
	{Hour, "%d minut %s", Minute},
	{2 * Hour, "1 godzinę %s", 1},
	{Day, "%d godzin %s", Hour},
	{2 * Day, "1 dzień %s", 1},
	{Week, "%d dni %s", Day},
	{2 * Week, "1 tydzień %s", 1},
	{Month, "%d tygodni %s", Week},
	{2 * Month, "1 miesiąc %s", 1},
	{Year, "%d miesięcy %s", Month},
	{18 * Month, "1 rok %s", 1},
	{2 * Year, "2 lata %s", 1},
	{LongTime, "%d lat %s", Year},
	{math.MaxInt64, "dobrą chwilę %s", 1},
}

func text(tpl string, params ...interface{}) string {
	return fmt.Sprintf(tpl, params...)
}
