package papaBot

import (
	"database/sql"
	"github.com/nickvanw/ircx"
	"github.com/op/go-logging"
	"time"
)

// Bot itself.
type Bot struct {
	irc *ircx.Bot
	Db  *sql.DB
	log *logging.Logger

	configFile string
	Config     Configuration

	BotOwner       string
	floodSemaphore chan int
	kickedFrom     map[string]bool

	commands        map[string]*botCommand
	commandUseLimit map[string]int
	commandWarn     map[string]bool

	urlProcessors []urlProcessor
	extensions    []extension

	textsFile string
	Texts     *botTexts

	lastURLAnnouncedTime        map[string]time.Time
	lastURLAnnouncedLinesPassed map[string]int
	urlMoreInfo                 map[string]string
}

// Interface for URL processors. They must implement these two methods.
type urlProcessor interface {
	Init(bot *Bot) error
	Process(bot *Bot, info *urlInfo, channel, sender, msg string)
}

// Extensions aren't explicitly run. They must inject themselves into bot's mechanisms (commands etc).
type extension interface {
	Init(bot *Bot) error
}

// Url information passed between url processors.
type urlInfo struct {
	link      string
	shortInfo string
	longInfo  string
}

// Bot's commands.
type botCommand struct {
	privateOnly bool
	ownerOnly   bool
	usage       string
	help        string
	commandFunc func(bot *Bot, nick, user, channel, receiver string, priv bool, params []string)
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
}

// Bot's core texts.
type botTexts struct {
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
