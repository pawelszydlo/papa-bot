package papaBot

// All structures used by bot (sans extensions and transports).

import (
	"database/sql"
	"github.com/pelletier/go-toml"
	"github.com/sirupsen/logrus"
	"net/http"
	"regexp"
	"time"

	"github.com/pawelszydlo/humanize"
	"github.com/pawelszydlo/papa-bot/events"
	"github.com/pawelszydlo/papa-bot/transports"
)

// Bot itself.
type Bot struct {
	// Was initialization complete?
	initDone bool
	// Database connection.
	Db *sql.DB
	// HTTP client.
	HTTPClient *http.Client
	// Logger.
	Log *logrus.Logger
	// Event dispatcher instance.
	EventDispatcher *events.EventDispatcher
	// Full config file tree.
	fullConfig *toml.Tree
	// Full texts file tree.
	fullTexts *toml.Tree
	// Bot's configuration.
	Config *Configuration
	// Bot texts struct.
	Texts *botTexts
	// Values humanizer.
	Humanizer *humanize.Humanizer
	// Currently authenticated users.
	authenticatedUsers  map[string]string
	authenticatedAdmins map[string]string
	authenticatedOwners map[string]string
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
	extensions []extension
	// Enabled transports.
	transports map[string]transports.Transport
	// Time when URL info was last announced, per channel + link.
	lastURLAnnouncedTime map[string]time.Time
	// Lines passed since URL info was last announced, per channel + link.
	lastURLAnnouncedLinesPassed map[string]int
	// More information to give about last link, per channel.
	urlMoreInfo map[string]string
	// Time for next daily tick.
	nextDailyTick time.Time
	// Regular expression for extracting sample text from website.
	webContentSampleRe *regexp.Regexp
}

// Interface representing an extension.
type extension interface {
	Init(bot *Bot) error
}

// Bot's commands.
type BotCommand struct {
	// Names of the command (main and aliases).
	CommandNames []string
	// Does this command require private query?
	Private bool
	// This command can only be run by the owner?
	Owner bool
	// This command can only be run by an admin?
	Admin bool
	// Help string showing possible parameters.
	HelpParams string
	// Help string with the description.
	HelpDescription string
	// Function to be executed.
	CommandFunc func(bot *Bot, sourceEvent *events.EventMessage, params []string)
}

// Bot's configuration. It will be loaded from the provided file on New(), overwriting any defaults.
type Configuration struct {
	Name                       string
	Language                   string
	ChatLogging                bool
	CommandsPer5               int
	UrlAnnounceIntervalMinutes time.Duration
	UrlAnnounceIntervalLines   int
	PageBodyMaxSize            uint
	HttpDefaultUserAgent       string
	DailyTickHour              int
	DailyTickMinute            int
	LogLevel                   logrus.Level
}

// Bot's core texts.
type botTexts struct {
	NeedsPriv           string
	NeedsAdmin          string
	PasswordOk          string
	SearchResults       string
	SearchNoResults     string
	SearchPrivateNotice string
	CommandLimit        string
	NothingToAdd        string
	WrongCommand        []string
}
