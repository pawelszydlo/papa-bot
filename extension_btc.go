package papaBot

import (
	"fmt"
	"math"
	"strconv"
	"text/template"
	"time"
)

type btcExtensionTextsStruct struct {
	NothingHasChanged string
	TplBtcNotice      string
	TempBtcNotice     *template.Template
}

var (
	btcHourlyLastQuery time.Time
	btcHourlyData      map[string]interface{}
	btcAsk             = map[string]time.Time{}
	btcWarned          = map[string]bool{}

	btcTexts = &btcExtensionTextsStruct{}
)

func initBtcExtension(bot *Bot) {
	// Register new command
	bot.commands["btc"] = &botCommand{
		false, false,
		"btc", "Show current BTC price.",
		bot.commandBtc}
	bot.loadTexts(bot.textsFile, btcTexts)
}

func (bot *Bot) commandBtc(nick, user, channel, receiver string, priv bool, params []string) {
	// Get fresh data max every 5 minutes.
	if time.Since(btcHourlyLastQuery) > 5*time.Minute {
		data, err := GetJsonResponse("http://www.bitstamp.net/api/ticker/")
		if err != nil {
			bot.log.Warning("Error getting BTC data: %s", err)
		}
		btcHourlyData = data
		btcHourlyLastQuery = time.Now()
	}
	// Answer only once per 5 minutes per channel.
	if time.Since(btcAsk[channel]) > 5*time.Minute {
		btcAsk[channel] = time.Now()
		btcWarned[channel] = false
		price, _ := strconv.ParseFloat(btcHourlyData["last"].(string), 64)
		//		open, _ := strconv.ParseFloat(btcHourlyData["open"].(string), 64)
		diff := price - btcHourlyData["open"].(float64)
		diffstr := ""
		if diff > 0 {
			diffstr = fmt.Sprintf("⬆$%.2f", math.Abs(diff))
		} else {
			diffstr = fmt.Sprintf("⬇$%.2f", math.Abs(diff))
		}
		pricestr := fmt.Sprintf("$%.2f", price)

		bot.SendNotice(receiver, Format(btcTexts.TempBtcNotice, &map[string]string{"price": pricestr, "diff": diffstr}))
	} else {
		// Only warn once.
		if !btcWarned[channel] {
			bot.SendMessage(receiver, fmt.Sprintf("%s, %s", nick, btcTexts.NothingHasChanged))
			btcWarned[channel] = true
		}
	}

}
