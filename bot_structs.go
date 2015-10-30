package papaBot

import (
	"database/sql"
	"github.com/nickvanw/ircx"
	"log"
	"text/template"
)

type Bot struct {
	irc        *ircx.Bot
	configFile string
	Config     Configuration
	BotOwner   string

	Db *sql.DB

	floodSemaphore chan int

	logDebug *log.Logger
	logInfo  *log.Logger
	logWarn  *log.Logger
	logError *log.Logger

	kickedFrom      map[string]bool
	commands        map[string]botCommand
	commandUseLimit map[string]int

	textsFile string
	Texts     botTexts
}

type Configuration struct {
	Server         string
	Name           string
	User           string
	OwnerPassword  string
	Channels       []string
	AntiFloodDelay int
	CommandsPer5   int
	LogChannel     bool
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
	Hellos              []string
	HellosAfterKick     []string
	WrongCommand        []string
}
