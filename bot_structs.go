package papaBot

import (
	"database/sql"
	"github.com/nickvanw/ircx"
	"github.com/op/go-logging"
	"text/template"
	"time"
)

type Bot struct {
	irc        *ircx.Bot
	configFile string
	Config     Configuration
	BotOwner   string

	Db *sql.DB

	floodSemaphore chan int

	log *logging.Logger

	kickedFrom      map[string]bool
	commands        map[string]botCommand
	commandUseLimit map[string]int

	textsFile string
	Texts     botTexts

	lastURLAnnouncedTime        map[string]time.Time
	lastURLAnnouncedLinesPassed map[string]int
	urlMoreInfo                 map[string]string
}

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
}

type botCommand struct {
	privateOnly bool
	ownerOnly   bool
	usage       string
	help        string
	commandFunc func(nick, user, channel, receiver string, priv bool, params []string)
}

type botTexts struct {
	tempDuplicateFirst  *template.Template
	DuplicateFirst      string
	tempDuplicateMulti  *template.Template
	DuplicateMulti      string
	DuplicateYou        string
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

type urlInfo struct {
	link      string
	shortInfo string
	longInfo  string
}
