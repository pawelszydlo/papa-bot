package extensions

import (
	"encoding/json"
	"fmt"
	"github.com/pawelszydlo/papa-bot"
	"net/url"
	"strings"
)

/*
ExtensionAqicn - query Aqicn.org for air quality data.

Used custom variables:
- AqicnToken- Your Aqicn token.
*/
type ExtensionAqicn struct {
	bot *papaBot.Bot
}

// Structs for Aqicn responses.
type aqiSearchResult struct {
	Status string
	Data   []aqiSearchData
}
type aqiSearchData struct {
	Uid int
}

type aqiQueryResult struct {
	Status string
	Data   aqiData
}
type aqiData struct {
	Aqi  int
	City aqiCity
	Iaqi aqiIaqi
}
type aqiCity struct {
	Name string
}
type aqiIaqi struct {
	No2  aqiValue
	O3   aqiValue
	Pm10 aqiValue
	Pm25 aqiValue
}
type aqiValue struct {
	V float64
}

// Init inits the extension.
func (ext *ExtensionAqicn) Init(bot *papaBot.Bot) error {
	// Register new command.
	bot.RegisterCommand(&papaBot.BotCommand{
		[]string{"aq"},
		false, false, false,
		"<station>", "Show air quality for <station>.",
		ext.commandAqicn})
	ext.bot = bot
	return nil
}

//qualityIndex shows how the value qualifies.
func (ext *ExtensionAqicn) qualityIndexLevel(stat string, value float64) int {
	norms := map[string][]int{
		"pm25": {15, 30, 55, 110},
		"pm10": {25, 50, 90, 180},
		"o3":   {60, 120, 180, 240},
		"no2":  {50, 100, 200, 400},
		"aqi": {50, 100, 150, 200},
	}
	for i, normValue := range norms[stat] {
		if int(value) < normValue {
			return i
		}
	}
	return 4
}

// interpretQualityIndex will put the quality index into human readable form.
func (ext *ExtensionAqicn) interpretQualityIndex(stat string, value float64) string {
	level := ext.qualityIndexLevel(stat, value)
	levels := map[int]string{
		0: ":smile:",
		1: ":slightly_smiling_face:",
		2: ":confused:",
		3: ":weary:",
		4: ":skull_and_crossbones:",
	}
	return levels[level]
}

// format is a helper function that will prepare a markdown value.
func (ext *ExtensionAqicn) format(stat string, value float64) string {
	if value == 0 { // no readout.
		return "- |"
	}
	return fmt.Sprintf("%.f %s |",value, ext.interpretQualityIndex(stat, value))
}

// queryAqicn will query aqicn.org first for stations matching "city", then for results for those stations.
func (ext *ExtensionAqicn) queryAqicn(city, transport string) string {
	token := ext.bot.GetVar("aqicnToken")
	if token == "" {
		ext.bot.Log.Errorf("Aqicn.org Token key not set! Set the 'aqicnToken' variable in the bot.")
	}

	err, _, body := ext.bot.GetPageBody(
		fmt.Sprintf(
			"https://api.waqi.info/search/?token=%s&keyword=%s",
			token, strings.Replace(url.QueryEscape(city), "+", "%20", -1),
		), nil)
	if err != nil {
		ext.bot.Log.Errorf("Error getting Aqicn data: %s", err)
		return ""
	}

	searchResult := aqiSearchResult{Status: "", Data: []aqiSearchData{}}
	// Decode JSON.
	if err := json.Unmarshal(body, &searchResult); err != nil {
		ext.bot.Log.Errorf("Error loading Aqicn.org data for %s: %s", city, err)
		return ""
	}

	// Check response.
	if len(searchResult.Data) == 0 {
		return ext.bot.Texts.SearchNoResults
	} else {
		ext.bot.Log.Infof("Found %d stations for city '%s'.", len(searchResult.Data), city)
	}

	// Gather data for each station.
	result := []string{}
	if transport == "mattermost" {
		result = append(result, "\n\n| Station | AQI | PM2.5 | PM10 | O3 | NO2 |")
		result = append(result, "| -----: | :----: | :----: | :----:| :----: | :----: | :----: |")
	}
	for _, station := range searchResult.Data {
		url := fmt.Sprintf("http://api.waqi.info/feed/@%d/?token=%s", station.Uid, token)
		ext.bot.Log.Warnf(url)
		err, _, body := ext.bot.GetPageBody(
			fmt.Sprintf("http://api.waqi.info/feed/@%d/?token=%s", station.Uid, token), nil)
		if err != nil {
			ext.bot.Log.Errorf("Error getting Aqicn data: %s", err)
			continue
		}
		queryResult := aqiQueryResult{"", aqiData{City: aqiCity{}, Iaqi: aqiIaqi{}}}
		// Decode JSON.
		if err := json.Unmarshal(body, &queryResult); err != nil {
			ext.bot.Log.Errorf("Error loading Aqicn.org data for %d: %s", station.Uid, err)
		} else {
			if transport == "mattermost" {
				line := fmt.Sprintf("| %s | ", queryResult.Data.City.Name)
				line += ext.format("aqi", float64(queryResult.Data.Aqi))
				line += ext.format("pm25", float64(queryResult.Data.Iaqi.Pm25.V))
				line += ext.format("pm10", float64(queryResult.Data.Iaqi.Pm10.V))
				line += ext.format("o3", float64(queryResult.Data.Iaqi.O3.V))
				line += ext.format("no2", float64(queryResult.Data.Iaqi.No2.V))
				result = append(result, line)
			} else {
				result = append(result, fmt.Sprintf(
					"%s - AQI: %d, PM10: %.f, PM25: %.f",
					queryResult.Data.City.Name, queryResult.Data.Aqi,
					queryResult.Data.Iaqi.Pm10.V, queryResult.Data.Iaqi.Pm25.V),
				)
			}
		}
	}
	if transport == "mattermost" {
		return strings.Join(result, "\n")
	}
	return strings.Join(result, " | ")
}

// commandMovie is a command for manually searching for movies.
func (ext *ExtensionAqicn) commandAqicn(bot *papaBot.Bot, nick, user, channel, transport, context string, priv bool, params []string) {
	if len(params) < 1 {
		return
	}
	search := strings.Join(params, " ")
	result := ext.queryAqicn(search, transport)

	bot.SendAutoMessage(priv, transport, nick, channel, result, context)
}
