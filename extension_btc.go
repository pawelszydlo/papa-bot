package papaBot

import (
	"encoding/json"
	"fmt"
	"github.com/pawelszydlo/papa-bot/utils"
	"math"
	"strconv"
	"text/template"
	"time"
)

// ExtensionBtc - extension for getting BTC price from BitStamp.com.
type ExtensionBtc struct {
	Extension
	HourlyData map[string]interface{}

	LastAsk map[string]time.Time
	Warned  map[string]bool

	priceSeries          []float64
	seriousChangePercent float64

	Texts *extensionBtcTexts
}

type extensionBtcTexts struct {
	NothingHasChanged  string
	TplBtcNotice       string
	TempBtcNotice      *template.Template
	TplBtcSeriousRise  string
	TempBtcSeriousRise *template.Template
	TplBtcSeriousFall  string
	TempBtcSeriousFall *template.Template
}

func (ext *ExtensionBtc) Init(bot *Bot) error {
	// Register new command.
	cmdBtc := BotCommand{
		false, false, false,
		"btc", "Show current BTC price.",
		ext.commandBtc}
	bot.commands["btc"] = &cmdBtc
	bot.commands["kierda"] = &cmdBtc
	// Init variables.
	ext.LastAsk = map[string]time.Time{}
	ext.Warned = map[string]bool{}
	ext.seriousChangePercent = 5
	ext.priceSeries = make([]float64, 12, 12)
	// Load texts.
	texts := new(extensionBtcTexts)
	if err := bot.LoadTexts(bot.textsFile, texts); err != nil {
		return err
	}
	ext.Texts = texts
	return nil
}

// diffStr will get a string representing the rise/fall of price.
func (ext *ExtensionBtc) diffStr(diff float64) string {
	diffstr := ""
	if diff > 0 {
		diffstr = fmt.Sprintf("⬆$%.2f", math.Abs(diff))
	} else {
		diffstr = fmt.Sprintf("⬇$%.2f", math.Abs(diff))
	}
	return diffstr
}

// Tick will monitor BTC price and warn if anything serious happens.
func (ext *ExtensionBtc) Tick(bot *Bot, daily bool) {
	// Fetch fresh data.
	body, err := bot.getPageBodyByURL("https://www.bitstamp.net/api/ticker/")
	if err != nil {
		bot.log.Warning("Error getting BTC data: %s", err)
		return
	}

	// Convert from JSON
	var raw_data interface{}
	if err := json.Unmarshal(body, &raw_data); err != nil {
		bot.log.Warning("Error parsing JSON from Bitstamp: %s", err)
		return
	}
	data := raw_data.(map[string]interface{})
	ext.HourlyData = data

	// Get current price.
	price, err := strconv.ParseFloat(data["last"].(string), 64)
	if err != nil {
		bot.log.Warning("Error in the BTC ticker: %s", err)
		return
	}

	// On daily tick, announce.
	if daily {
		diff := price - ext.HourlyData["open"].(float64)

		bot.SendMassNotice(utils.Format(ext.Texts.TempBtcNotice, map[string]string{
			"price": fmt.Sprintf("$%.2f", price),
			"diff":  ext.diffStr(diff)}))
	}

	// Append to the FIFO series.
	ext.priceSeries = ext.priceSeries[1:]
	ext.priceSeries = append(ext.priceSeries, price)

	// Check if we have a serious change
	min_price := 99999.9 // Hopefully soon we'll have to use math.MaxFloat64
	max_price := 0.0
	min_index := -1
	max_index := -1
	for i, v := range ext.priceSeries {
		if v == 0.0 {
			continue
		}
		if v >= max_price {
			max_price = v
			max_index = i
		}
		if v < min_price {
			min_price = v
			min_index = i
		}
	}
	// Was anything found?
	if min_index != max_index {
		diff := max_price - min_price
		rise := min_index < max_index
		time_diff := math.Abs(float64(max_index)-float64(min_index)) * 5
		// Announce threshold.
		thresh := float64(ext.seriousChangePercent) / 100 * max_price
		if rise {
			thresh = float64(ext.seriousChangePercent) / 100 * min_price
		}
		if diff > thresh {
			values := map[string]string{
				"diff": "", "minutes": fmt.Sprintf("%.0f", time_diff), "price": fmt.Sprintf("$%.2f", price)}
			if rise {
				values["diff"] = ext.diffStr(diff)
				bot.SendMassNotice(utils.Format(ext.Texts.TempBtcSeriousRise, values))
			} else {
				values["diff"] = ext.diffStr(-diff)
				bot.SendMassNotice(utils.Format(ext.Texts.TempBtcSeriousFall, values))
			}
			ext.priceSeries = make([]float64, 12, 12) // Empty the series.
		}
	}
}

func (ext *ExtensionBtc) commandBtc(bot *Bot, nick, user, channel, receiver string, priv bool, params []string) {
	// Answer only once per 5 minutes per channel.
	if time.Since(ext.LastAsk[channel]) > 5*time.Minute {
		ext.LastAsk[channel] = time.Now()
		ext.Warned[channel] = false
		price, _ := strconv.ParseFloat(ext.HourlyData["last"].(string), 64)
		diff := price - ext.HourlyData["open"].(float64)

		bot.SendNotice(receiver, utils.Format(ext.Texts.TempBtcNotice, map[string]string{
			"price": fmt.Sprintf("$%.2f", price),
			"diff":  ext.diffStr(diff)}))
	} else {
		// Only warn once.
		if !ext.Warned[channel] {
			bot.SendMessage(receiver, fmt.Sprintf("%s, %s", nick, ext.Texts.NothingHasChanged))
			ext.Warned[channel] = true
		}
	}

}

// Not implemented.
func (ext *ExtensionBtc) ProcessURL(bot *Bot, urlinfo *UrlInfo, channel, sender, msg string) {}
func (ext *ExtensionBtc) ProcessMessage(bot *Bot, channel, sender, msg string)               {}
