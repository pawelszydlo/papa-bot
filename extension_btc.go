package papaBot

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"text/template"
	"time"
)

// ExtensionBtc - extension for getting BTC price from BitStamp.com.
type ExtensionBtc struct {
	HourlyData map[string]interface{}
	LastAsk    map[string]time.Time
	Warned     map[string]bool

	lastHalfHour         []float64
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
	bot.commands["btc"] = &BotCommand{
		false, false,
		"btc", "Show current BTC price.",
		ext.commandBtc}
	// Init variables.
	ext.LastAsk = map[string]time.Time{}
	ext.Warned = map[string]bool{}
	ext.seriousChangePercent = 10
	ext.lastHalfHour = make([]float64, 6, 6)
	// Load texts.
	texts := new(extensionBtcTexts)
	if err := bot.LoadTexts(bot.textsFile, texts); err != nil {
		return err
	}
	ext.Texts = texts
	return nil
}

func (ext *ExtensionBtc) ProcessURL(bot *Bot, urlinfo *UrlInfo, channel, sender, msg string) {}

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
	body, err := bot.GetPageBodyByURL("http://www.bitstamp.net/api/ticker/")
	if err != nil {
		bot.log.Warning("Error getting BTC data: %s", err)
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

	// Append to the FIFO series.
	ext.lastHalfHour = ext.lastHalfHour[1:]
	ext.lastHalfHour = append(ext.lastHalfHour, price)

	// Check if we have a serious change
	min_price := 99999.9 // Hopefully soon we'll have to use math.MaxFloat64
	max_price := 0.0
	min_index := 9
	max_index := 9
	for i, v := range ext.lastHalfHour {
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
	if min_index != 9 {
		diff := max_price - min_price
		time_diff := math.Abs(float64(max_index)-float64(min_index)) * 5
		if math.Abs(diff) > float64(ext.seriousChangePercent)/100*max_price {
			values := map[string]string{
				"diff": "", "minutes": fmt.Sprintf("%.0f", time_diff), "price": fmt.Sprintf("$%.0f", price)}
			if max_index > min_index {
				values["diff"] = ext.diffStr(diff)
				bot.SendMassNotice(Format(ext.Texts.TempBtcSeriousRise, &values))
			} else if min_index > max_index {
				values["diff"] = ext.diffStr(-diff)
				bot.SendMassNotice(Format(ext.Texts.TempBtcSeriousFall, &values))
			}
			ext.lastHalfHour = make([]float64, 6, 6) // Empty the series.
		}
	}
}

func (ext *ExtensionBtc) commandBtc(bot *Bot, nick, user, channel, receiver string, priv bool, params []string) {
	// Answer only once per 5 minutes per channel.
	if time.Since(ext.LastAsk[channel]) > 5*time.Minute {
		ext.LastAsk[channel] = time.Now()
		ext.Warned[channel] = false
		price, _ := strconv.ParseFloat(ext.HourlyData["last"].(string), 64)
		//		open, _ := strconv.ParseFloat(btcHourlyData["open"].(string), 64)
		diff := price - ext.HourlyData["open"].(float64)
		diffstr := ext.diffStr(diff)
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
