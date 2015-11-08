package papaBot

// All structures used by bot (sans extensions).

import (
	"database/sql"
	"github.com/nickvanw/ircx"
	"github.com/op/go-logging"
	"net/http"
	"time"
)

// Bot itself.
type Bot struct {
	// Underlying irc bot.
	irc *ircx.Bot
	// Database connection.
	Db *sql.DB
	// HTTP client.
	HTTPClient *http.Client
	// Logger.
	log *logging.Logger
	// Path to config file.
	configFile string
	// Configuration struct.
	Config Configuration
	// Anti flood buffered semaphore
	floodSemaphore chan int
	// Channels bot was kicked from.
	kickedFrom map[string]bool
	// Currently authenticated users.
	authenticatedAdmins map[string]bool
	authenticatedOwners map[string]bool
	// Registered bot commands.
	commands map[string]*BotCommand
	// Number of uses per command.
	commandUseLimit map[string]int
	// Was the warning sent, per command.
	commandWarn map[string]bool
	// Commands that will not have their params listed in the logs (auth etc.)
	commandsHideParams map[string]bool
	// Custom variables for use in extensions.
	customVars map[string]string
	// Registered bot extensions,
	extensions []Extension
	// Path to texts file.
	textsFile string
	// Bot texts struct.
	Texts *BotTexts
	// Time when URL info was last announced, per channel + link.
	lastURLAnnouncedTime map[string]time.Time
	// Lines passed since URL info was last announced, per channel + link.
	lastURLAnnouncedLinesPassed map[string]int
	// More information to give about last link, per channel.
	urlMoreInfo map[string]string
	// Time for next daily tick.
	nextDailyTick time.Time
}

// Extensions must implement these methods.
type Extension interface {
	// Will be run on bot's init.
	Init(bot *Bot) error
	// Will be run whenever an URL is found in the message.
	ProcessURL(bot *Bot, info *UrlInfo, channel, sender, msg string)
	// Will be run on every public message the bot receives.
	ProcessMessage(bot *Bot, channel, nick, msg string)
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
	// This command can only be run by an admin?
	Admin bool
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
	Language                   string
	Channels                   []string
	AntiFloodDelay             int
	CommandsPer5               int
	LogChannel                 bool
	UrlAnnounceIntervalMinutes time.Duration
	UrlAnnounceIntervalLines   int
	RejoinDelaySeconds         time.Duration
	PageBodyMaxSize            uint
	HttpDefaultUserAgent       string
	DailyTickHour              int
	DailyTickMinute            int
}

// Bot's core texts.
type BotTexts struct {
	NeedsPriv           string
	NeedsAdmin          string
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
