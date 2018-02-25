package extensions

import (
	"encoding/json"
	"fmt"
	"github.com/pawelszydlo/papa-bot"
	"net/url"
	"strings"
)

/* ExtensionMovies - finds movie titles in the messages and provides other movie related commands.

Used custom variables:
- moviesInMsg - string of "yes" or "no". Should the extension look for movie titles in conversation?
- moviesTriggerWords - list of space separated strings. Any of these words must be present in a text to trigger movie
                       title lookup.
*/
type ExtensionMovies struct {
	Extension
	announced map[string]bool
}

type movieStruct struct {
	Error      string
	Poster     string
	Runtime    string
	Director   string
	Actors     string
	Language   string
	Title      string
	Plot       string
	Country    string
	ImdbRating string
	ImdbID     string
	Type       string
	Year       string
	Genre      string
	ImdbVotes  string
}

// Init inits the extension.
func (ext *ExtensionMovies) Init(bot *papaBot.Bot) error {
	ext.announced = map[string]bool{}
	// Init variables.
	if doTrigger := bot.GetVar("moviesInMsg"); doTrigger == "" {
		bot.Log.Warningf("Movie title lookup in messages not set in 'moviesInContext' variable. Setting default.")
		bot.SetVar("moviesInMsg", "no")
	}
	if triggerWords := bot.GetVar("moviesTriggerWords"); triggerWords == "" {
		bot.Log.Warningf("No movie trigger words found in 'moviesTriggerWords' variable. Setting default.")
		bot.SetVar("moviesTriggerWords", "seen watched movie watching cinema movies")
	}
	bot.Log.Debugf("Look for movie titles in messages: %s", bot.GetVar("moviesInMsg"))
	bot.Log.Debugf("Movie trigger words set to: %s", bot.GetVar("moviesTriggerWords"))

	// Register new command.
	bot.RegisterCommand(&papaBot.BotCommand{
		[]string{"i", "imdb"},
		false, false, false,
		"<title>", "Get movie info for <title>.",
		ext.commandMovie})
	return nil
}

// searchOmdb will query Omdb database for movie information.
func (ext *ExtensionMovies) searchOmdb(bot *papaBot.Bot, title string, data *movieStruct) {
	// Fetch movie data.
	body, err := bot.GetPageBodyByURL("http://www.omdbapi.com/?y=&plot=short&r=json&t=" + url.QueryEscape(title))
	if err != nil {
		data.Error = fmt.Sprintf("%s", err)
		return
	}

	// Convert from JSON
	if err := json.Unmarshal(body, &data); err != nil {
		data.Error = fmt.Sprintf("%s", err)
		return
	}
}

// announce announces movie info to the channel.
func (ext *ExtensionMovies) findAndAnnounce(bot *papaBot.Bot, channel, title string) {
	// Announce each movie only once.
	if ext.announced[channel+title] {
		return
	}
	title = strings.Replace(title, `"`, "", -1)
	var data movieStruct
	ext.searchOmdb(bot, title, &data)
	if data.Error != "" {
		bot.Log.Debugf("Omdb error: %s", data.Error)
		return
	}
	// Omdb returns very strange results, sometimes for some obscure movies when a popular one exists
	if data.ImdbRating == "N/A" || data.ImdbVotes == "N/A" || data.Type == "episode" {
		bot.Log.Debugf("Not worth announcing movie: %s", data.Title)
		return
	}
	// Announce.
	notice := fmt.Sprintf("%s (%s, %s) | %s | http://www.imdb.com/title/%s | %s",
		data.Title, data.Genre, data.Year, data.ImdbRating, data.ImdbID, data.Plot)
	bot.SendNotice(channel, notice)
	ext.announced[channel+title] = true
}

// commandMovie is a command for manually searching for movies.
func (ext *ExtensionMovies) commandMovie(bot *papaBot.Bot, nick, user, channel, receiver string, priv bool, params []string) {
	if len(params) < 1 {
		return
	}
	title := strings.Join(params, " ")
	ext.findAndAnnounce(bot, receiver, title)
}
