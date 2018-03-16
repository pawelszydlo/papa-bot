package extensions

import (
	"encoding/json"
	"fmt"
	"github.com/pawelszydlo/papa-bot"
	"github.com/pawelszydlo/papa-bot/events"
	"net/url"
	"strings"
)

/* ExtensionMovies - finds movie titles in the messages and provides other movie related commands. */

// TODO: check why this extension doesn't fetch data.
type ExtensionMovies struct {
	announced map[string]bool
	bot       *papaBot.Bot
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

	// Register new command.
	bot.RegisterCommand(&papaBot.BotCommand{
		[]string{"i", "imdb"},
		false, false, false,
		"<title>", "Get movie info for <title>.",
		ext.commandMovie})
	ext.bot = bot
	return nil
}

// searchOmdb will query Omdb database for movie information.
func (ext *ExtensionMovies) searchOmdb(bot *papaBot.Bot, title string, data *movieStruct) {
	// Fetch movie data.
	err, _, body := ext.bot.GetPageBody("http://www.omdbapi.com/?y=&plot=short&r=json&t="+url.QueryEscape(title), nil)
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

// commandMovie is a command for manually searching for movies.
func (ext *ExtensionMovies) commandMovie(bot *papaBot.Bot, sourceEvent *events.EventMessage, params []string) {
	if len(params) < 1 {
		return
	}
	title := strings.Join(params, " ")
	// Announce each movie only once.
	if ext.announced[sourceEvent.Channel+title] {
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
	bot.SendNotice(sourceEvent, notice)
	ext.announced[sourceEvent.Channel+title] = true
}
