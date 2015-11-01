package papaBot

import (
	"fmt"
	"math"
	"strconv"
	"text/template"
	"time"
)

type ExtensionBtc struct {
	HourlyLastQuery time.Time
	HourlyData      map[string]interface{}
	LastAsk         map[string]time.Time
	Warned          map[string]bool

	Texts *extensionBtcTexts
}

type extensionBtcTexts struct {
	NothingHasChanged string
	TplBtcNotice      string
	TempBtcNotice     *template.Template
}

func (ext *ExtensionBtc) Init(bot *Bot) error {
	// Register new command.
	bot.commands["btc"] = &botCommand{
		false, false,
		"btc", "Show current BTC price.",
		ext.commandBtc}
	ext.LastAsk = map[string]time.Time{}
	ext.Warned = map[string]bool{}
	texts := new(extensionBtcTexts)
	if err := bot.loadTexts(bot.textsFile, texts); err != nil {
		return err
	}
	ext.Texts = texts
	return nil
}

func (ext *ExtensionBtc) commandBtc(bot *Bot, nick, user, channel, receiver string, priv bool, params []string) {
	// Get fresh data max every 5 minutes.
	if time.Since(ext.HourlyLastQuery) > 5*time.Minute {
		data, err := GetJsonResponse("http://www.bitstamp.net/api/ticker/")
		if err != nil {
			bot.log.Warning("Error getting BTC data: %s", err)
		}
		ext.HourlyData = data
		ext.HourlyLastQuery = time.Now()
	}
	// Answer only once per 5 minutes per channel.
	if time.Since(ext.LastAsk[channel]) > 5*time.Minute {
		ext.LastAsk[channel] = time.Now()
		ext.Warned[channel] = false
		price, _ := strconv.ParseFloat(ext.HourlyData["last"].(string), 64)
		//		open, _ := strconv.ParseFloat(btcHourlyData["open"].(string), 64)
		diff := price - ext.HourlyData["open"].(float64)
		diffstr := ""
		if diff > 0 {
			diffstr = fmt.Sprintf("⬆$%.2f", math.Abs(diff))
		} else {
			diffstr = fmt.Sprintf("⬇$%.2f", math.Abs(diff))
		}
		pricestr := fmt.Sprintf("$%.2f", price)

		bot.SendNotice(receiver, Format(ext.Texts.TempBtcNotice, &map[string]string{"price": pricestr, "diff": diffstr}))
	} else {
		// Only warn once.
		if !ext.Warned[channel] {
			bot.SendMessage(receiver, fmt.Sprintf("%s, %s", nick, ext.Texts.NothingHasChanged))
			ext.Warned[channel] = true
		}
	}

}
