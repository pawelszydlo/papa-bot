package papaBot

import (
	"encoding/json"
	"fmt"
	"github.com/pawelszydlo/papa-bot/lexical"
	"net/url"
	"strings"
)

// ExtensionMovies - finds movie titles in the messages and provides other movie related commands.
type extensionMovies struct {
	Extension
	announced map[string]bool
	Texts     *extensionMoviesTexts
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

type extensionMoviesTexts struct {
	MoviesTriggerWords []string
}

// Init inits the extension.
func (ext *extensionMovies) Init(bot *Bot) error {
	// Load texts.
	ext.announced = map[string]bool{}
	texts := &extensionMoviesTexts{}
	if err := bot.LoadTexts(bot.textsFile, texts); err != nil {
		return err
	}
	ext.Texts = texts
	// Register new command.
	bot.RegisterCommand(&BotCommand{
		[]string{"i", "imdb"},
		false, false, false,
		"[title]", "Get movie info for [title].",
		ext.commandMovie})
	return nil
}

// searchOmdb will query Omdb database for movie information.
func (ext *extensionMovies) searchOmdb(bot *Bot, title string, data *movieStruct) {
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
func (ext *extensionMovies) findAndAnnounce(bot *Bot, channel, title string) {
	// Announce each movie only once.
	if ext.announced[channel+title] {
		return
	}
	title = strings.Replace(title, `"`, "", -1)
	var data movieStruct
	ext.searchOmdb(bot, title, &data)
	if data.Error != "" {
		bot.log.Debug("Omdb error: %s", data.Error)
		return
	}
	// Omdb returns very strange results, sometimes for some obscure movies when a popular one exists
	if data.ImdbRating == "N/A" || data.ImdbVotes == "N/A" || data.Type == "episode" {
		bot.log.Debug("Not worth announcing movie: %s", data.Title)
		return
	}
	// Announce.
	ext.announced[channel+title] = true
	notice := fmt.Sprintf("%s (%s, %s) | %s | http://www.imdb.com/title/%s | %s",
		data.Title, data.Genre, data.Year, data.ImdbRating, data.ImdbID, data.Plot)
	bot.SendNotice(channel, notice)
	ext.announced[channel+title] = true
}

// commandMovie is a command for manually searching for movies.
func (ext *extensionMovies) commandMovie(bot *Bot, nick, user, channel, receiver string, priv bool, params []string) {
	if len(params) < 1 {
		return
	}
	title := strings.Join(params, " ")
	ext.findAndAnnounce(bot, receiver, title)
}

// ProcessMessage will fetch information on movies mentioned in the post.
func (ext *extensionMovies) ProcessMessage(bot *Bot, channel, sender, msg string) {
	// Check if the message has any of the trigger words.
	hasTrigger := false
	for _, word := range ext.Texts.MoviesTriggerWords {
		if strings.Contains(msg, word) {
			hasTrigger = true
			break
		}
	}
	if !hasTrigger {
		return
	}
	names := lexical.FindQuotes(msg)
	for _, title := range names {
		ext.findAndAnnounce(bot, channel, title)
	}
}
