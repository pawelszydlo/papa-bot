package papaBot

import (
	"database/sql"
	"github.com/nickvanw/ircx"
	"github.com/op/go-logging"
	"net/http"
	"time"
)

// Bot itself.
type Bot struct {
	irc        *ircx.Bot
	Db         *sql.DB
	HTTPClient *http.Client
	log        *logging.Logger

	configFile string
	Config     Configuration

	BotOwner       string
	floodSemaphore chan int
	kickedFrom     map[string]bool

	commands        map[string]*BotCommand
	commandUseLimit map[string]int
	commandWarn     map[string]bool

	extensions []Extension

	textsFile string
	Texts     *BotTexts

	lastURLAnnouncedTime        map[string]time.Time
	lastURLAnnouncedLinesPassed map[string]int
	urlMoreInfo                 map[string]string
}

// Extensions must implement these methods.
type Extension interface {
	// Will be run on bot's init.
	Init(bot *Bot) error
	// Will be run whenever an URL is found in the message.
	ProcessURL(bot *Bot, info *UrlInfo, channel, sender, msg string)
	// Will be run every 5 minutes. Daily will be set to true once per day.
	Tick(bot *Bot, daily bool)
}

// Url information passed between url processors.
type UrlInfo struct {
	// The URL itself.
	URL string
	// Title (this is a bit special, with one extension in mind).
	Title string
	// Content type (if available).
	ContentType string
	// Body (will be available only for type/html and type/text).
	Body []byte
	// Short info will be sent as a notice to the channel immediately.
	ShortInfo string
	// Long info will be saved as "more".
	LongInfo string
}

// Bot's commands.
type BotCommand struct {
	// Does this command require private query?
	Private bool
	// This command can only be run by the owner?
	Owner bool
	// Help string showing the usage.
	HelpUsage string
	// Help string with the description.
	HelpDescription string
	// Function to be executed.
	CommandFunc func(bot *Bot, nick, user, channel, receiver string, priv bool, params []string)
}

// Bot's configuration.
type Configuration struct {
	Server                     string
	Name                       string
	User                       string
	OwnerPassword              string
	Channels                   []string
	AntiFloodDelay             int
	CommandsPer5               int
	LogChannel                 bool
	UrlAnnounceIntervalMinutes time.Duration
	UrlAnnounceIntervalLines   int
	RejoinDelaySeconds         time.Duration
	PageBodyMaxSize            uint
	HttpDefaultUserAgent       string
}

// Bot's core texts.
type BotTexts struct {
	NeedsPriv           string
	NeedsOwner          string
	PasswordOk          string
	SearchResults       string
	SearchNoResults     string
	SearchPrivateNotice string
	CommandLimit        string
	NothingToAdd        string
	Hellos              []string
	HellosAfterKick     []string
	WrongCommand        []string
}
