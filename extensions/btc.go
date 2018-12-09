package extensions

import (
	"encoding/json"
	"fmt"
	"github.com/pawelszydlo/papa-bot"
	"github.com/pawelszydlo/papa-bot/events"
	"github.com/pawelszydlo/papa-bot/utils"
	"math"
	"strconv"
	"text/template"
	"time"
)

// ExtensionBtc - extension for getting BTC price from BitStamp.com.
type ExtensionBtc struct {
	HourlyData map[string]interface{}

	LastAsk map[string]time.Time
	Warned  map[string]bool

	priceSeries          []float64
	seriousChangePercent float64

	Texts *extensionBtcTexts

	bot *papaBot.Bot
}

type extensionBtcTexts struct {
	NothingHasChanged  string
	NoData             string
	TempBtcNotice      *template.Template
	TempBtcSeriousRise *template.Template
	TempBtcSeriousFall *template.Template
}

func (ext *ExtensionBtc) Init(bot *papaBot.Bot) error {
	// Register new command.
	bot.RegisterCommand(&papaBot.BotCommand{
		[]string{"b", "btc", "k", "kierda"},
		false, false, false,
		"", "Show current BTC price.",
		ext.commandBtc})
	// Init variables.
	ext.LastAsk = map[string]time.Time{}
	ext.Warned = map[string]bool{}
	ext.seriousChangePercent = 5
	ext.priceSeries = make([]float64, 12, 12)
	ext.bot = bot
	// Load texts.
	texts := new(extensionBtcTexts)
	if err := bot.LoadTexts("btc", texts); err != nil {
		return err
	}
	ext.Texts = texts
	// Attach to events.
	bot.EventDispatcher.RegisterListener(events.EventTick, ext.TickListener)
	bot.EventDispatcher.RegisterListener(events.EventDailyTick, ext.DailyTickListener)
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

func (ext *ExtensionBtc) simpleAnnounceMessage() string {
	if ext.HourlyData == nil {
		// No data yet received? This can happen only if bot didn't tick the extension!
		ext.bot.Log.Error("BTC extension wasn't ticked before if was asked a price!")
		return ext.Texts.NoData
	}

	price, _ := strconv.ParseFloat(ext.HourlyData["last"].(string), 64)
	diff := price - ext.HourlyData["open"].(float64)
	high, _ := strconv.ParseFloat(ext.HourlyData["high"].(string), 64)
	low, _ := strconv.ParseFloat(ext.HourlyData["low"].(string), 64)

	return utils.Format(ext.Texts.TempBtcNotice, map[string]string{
		"price": fmt.Sprintf("$%.0f", price),
		"diff":  ext.diffStr(diff),
		"low":   fmt.Sprintf("$%.0f", low),
		"high":  fmt.Sprintf("$%.0f", high),
	})
}

// DailyTickListener announce the price.
func (ext *ExtensionBtc) DailyTickListener(message events.EventMessage) {
	ext.bot.SendMassNotice(ext.simpleAnnounceMessage())
}

// TickListener will monitor BTC price and warn if anything serious happens.
func (ext *ExtensionBtc) TickListener(message events.EventMessage) {
	// Fetch fresh data.
	err, _, body := ext.bot.GetPageBody("https://www.bitstamp.net/api/ticker/", nil)
	if err != nil {
		ext.bot.Log.Warningf("Error getting BTC data: %s", err)
		return
	}

	// Convert from JSON
	var raw_data interface{}
	if err := json.Unmarshal(body, &raw_data); err != nil {
		ext.bot.Log.Warningf("Error parsing JSON from Bitstamp: %s", err)
		return
	}
	data := raw_data.(map[string]interface{})
	ext.HourlyData = data

	// Get current price.
	price, err := strconv.ParseFloat(data["last"].(string), 64)
	if err != nil {
		ext.bot.Log.Warningf("Error in the BTC ticker: %s", err)
		return
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
				ext.bot.SendMassNotice(utils.Format(ext.Texts.TempBtcSeriousRise, values))
			} else {
				values["diff"] = ext.diffStr(-diff)
				ext.bot.SendMassNotice(utils.Format(ext.Texts.TempBtcSeriousFall, values))
			}
			ext.priceSeries = make([]float64, 12, 12) // Empty the series.
		}
	}
}

func (ext *ExtensionBtc) commandBtc(bot *papaBot.Bot, sourceEvent *events.EventMessage, params []string) {
	// Answer only once per 5 minutes per channel.
	if time.Since(ext.LastAsk[sourceEvent.Channel]) > 5*time.Minute {
		ext.LastAsk[sourceEvent.Channel] = time.Now()
		ext.Warned[sourceEvent.Channel] = false
		bot.SendNotice(sourceEvent, ext.simpleAnnounceMessage())

	} else {
		// Only warn once.
		if !ext.Warned[sourceEvent.Channel] {
			bot.SendMessage(sourceEvent, fmt.Sprintf("%s, %s", sourceEvent.Nick, ext.Texts.NothingHasChanged))
			ext.Warned[sourceEvent.Channel] = true
		}
	}

}
