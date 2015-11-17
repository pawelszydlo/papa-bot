package papaBot

// All structures used by bot (sans extensions).

import (
	"crypto/tls"
	"database/sql"
	"github.com/op/go-logging"
	"github.com/sorcix/irc"
	"net"
	"net/http"
	"time"
)

// Bot itself.
type Bot struct {
	// Was initialization complete?
	initDone bool
	// IRC network connection.
	irc *ircConnection
	// Database connection.
	Db *sql.DB
	// HTTP client.
	HTTPClient *http.Client
	// Logger.
	log *logging.Logger
	// Path to config file.
	configFile string
	// Bot's configuration.
	Config *Configuration
	// Anti flood buffered semaphore
	floodSemaphore chan int
	// Channels bot was kicked from.
	kickedFrom map[string]bool
	// Channels the bot is on.
	onChannel map[string]bool
	// Currently authenticated users.
	authenticatedUsers  map[string]string
	authenticatedAdmins map[string]string
	authenticatedOwners map[string]string
	// Registered event handlers.
	eventHandlers map[string]func(msg *irc.Message)
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
	extensions []extensionInterface
	// Path to texts file.
	textsFile string
	// Bot texts struct.
	Texts *botTexts
	// Time when URL info was last announced, per channel + link.
	lastURLAnnouncedTime map[string]time.Time
	// Lines passed since URL info was last announced, per channel + link.
	lastURLAnnouncedLinesPassed map[string]int
	// More information to give about last link, per channel.
	urlMoreInfo map[string]string
	// Time for next daily tick.
	nextDailyTick time.Time
}

// Bot's connection to the network.
type ircConnection struct {
	// IRC messages stream.
	messages chan *irc.Message
	// Network connection.
	connection net.Conn
	// IO.
	decoder *irc.Decoder
	encoder *irc.Encoder
}

// Extensions should embed this struct and override any methods necessary.
type Extension struct{}

// Will be run on bot's init or when extension is registered after bot's init.
func (ext *Extension) Init(bot *Bot) error { return nil }

// Will be run whenever an URL is found in the message.
func (ext *Extension) ProcessURL(bot *Bot, info *UrlInfo, channel, sender, msg string) {}

// Will be run on every public message the bot receives.
func (ext *Extension) ProcessMessage(bot *Bot, channel, nick, msg string) {}

// Will be run every 5 minutes. Daily will be set to true once per day.
func (ext *Extension) Tick(bot *Bot, daily bool) {}

// Interface for easier handling of extensions.
type extensionInterface interface {
	Init(bot *Bot) error
	ProcessURL(bot *Bot, info *UrlInfo, channel, sender, msg string)
	ProcessMessage(bot *Bot, channel, nick, msg string)
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
	CommandFunc func(bot *Bot, nick, user, channel, receiver string, priv bool, params []string)
}

// Bot's configuration. It will be loaded from the provided file on New(), overwriting any defaults.
type Configuration struct {
	// Connection parameters
	Server    string
	Name      string
	User      string
	Password  string
	TLSConfig *tls.Config
	// Other options.
	Language                   string
	Channels                   []string
	AntiFloodDelay             int
	CommandsPer5               int
	ChatLogging                bool
	UrlAnnounceIntervalMinutes time.Duration
	UrlAnnounceIntervalLines   int
	RejoinDelay                time.Duration
	PageBodyMaxSize            uint
	HttpDefaultUserAgent       string
	DailyTickHour              int
	DailyTickMinute            int
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
	Hellos              []string
	HellosAfterKick     []string
	WrongCommand        []string
}
