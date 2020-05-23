package extensions

import (
	"encoding/json"
	"fmt"
	"github.com/pawelszydlo/papa-bot"
	"github.com/pawelszydlo/papa-bot/events"
	"net/url"
	"strings"
)

/* ExtensionMovies - finds movie titles in the messages and provides other movie related commands.

Used custom variables:
- omdbKey - Your AMDb API key.
*/

type ExtensionMovies struct {
	announced map[string]bool
	bot       *papaBot.Bot
}

type movieStruct struct {
	Error string

	Title    string
	Released string
	Year     string
	Type     string
	Genre    string
	Poster   string
	Runtime  string

	Plot       string
	ImdbRating string
	ImdbID     string
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
	key := ext.bot.GetVar("omdbKey")
	if key == "" {
		ext.bot.Log.Panic("OMDb API key not set! Set the 'omdbKey' variable in the bot.")
	}
	// Fetch movie data.
	headers := map[string]string{"User-Agent": "PapaBot version " + papaBot.Version}
	err, _, body := ext.bot.GetPageBody(
		fmt.Sprintf("http://www.omdbapi.com/?apikey=%s&plot=short&r=json&t=%s", key, url.QueryEscape(title)),
		headers)
	if err != nil {
		data.Error = fmt.Sprintf("Error getting data: %s", err)
		return
	}
	// Convert from JSON
	if err := json.Unmarshal(body, &data); err != nil {
		data.Error = fmt.Sprintf("Error parsing JSON: %s", err)
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
	if ext.announced[sourceEvent.ChannelId()+title] {
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
	ext.announced[sourceEvent.ChannelId()+title] = true
}
