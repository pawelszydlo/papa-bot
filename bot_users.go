package papaBot

// Functions regarding user handling.

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/mattn/go-sqlite3"
	"github.com/pawelszydlo/papa-bot/utils"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// ensureOwnerExists makes sure that at least one owner exists in the database.
func (bot *Bot) ensureOwnerExists() {
	result, err := bot.Db.Query(`SELECT EXISTS(SELECT 1 FROM users WHERE owner=1 LIMIT 1);`)
	if err != nil {
		bot.log.Fatal("Can't check if owner exists: %s", err)
	}
	defer result.Close()

	if result.Next() {
		var ownerExists bool
		if err = result.Scan(&ownerExists); err != nil {
			bot.log.Fatal("Can't check if owner exists: %s", err)
		}
		if !ownerExists {
			bot.log.Warning("No owner found in the database. Must create one.")

			stty, _ := exec.LookPath("stty")
			sttyArgs := syscall.ProcAttr{
				"",
				[]string{},
				[]uintptr{os.Stdin.Fd(), os.Stdout.Fd(), os.Stderr.Fd()},
				nil,
			}
			reader := bufio.NewReader(os.Stdin)

			fmt.Print("Enter owner's nick: ")
			nick, _ := reader.ReadString('\n')

			// Disable echo.
			if stty != "" {
				syscall.ForkExec(stty, []string{"stty", "-echo"}, &sttyArgs)
			}

			// Get password.
			fmt.Print("Enter owner's password: ")
			pass1, _ := reader.ReadString('\n')
			fmt.Print("\nConfirm owner's password: ")
			pass2, _ := reader.ReadString('\n')
			if pass1 != pass2 {
				bot.log.Fatal("Passwords don't match.")
			}
			fmt.Print("\n")

			// Enable echo.
			if stty != "" {
				syscall.ForkExec(stty, []string{"stty", "echo"}, &sttyArgs)
			}

			result.Close()

			if bot.addUser(utils.CleanString(nick, false), utils.CleanString(pass1, false), true, true); err != nil {
				bot.log.Critical("%s", err)
			}
		}
	}
}

// addUser adds new user to bot's database.
func (bot *Bot) addUser(nick, password string, owner, admin bool) error {
	if password == "" {
		return errors.New("Password can't be empty.")
	}
	// Insert user into the db.
	if _, err := bot.Db.Exec(`INSERT INTO users(nick, password, owner, admin) VALUES(?, ?, ?, ?)`,
		nick, utils.HashPassword(password), owner, admin); err != nil {
		// Get exact error.
		driverErr := err.(sqlite3.Error)
		if driverErr.Code == sqlite3.ErrConstraint {
			return errors.New("User already exists.")
		}
		return errors.New("Error while adding new user!")
	}
	return nil
}

// getUserData fetches user information from database.
func (bot *Bot) getUserData(nick string) (
	dbNick, password string, altNicks map[string]bool, owner, admin bool, err error) {

	altNicks = map[string]bool{}
	result, err := bot.Db.Query(`
		SELECT nick, password, IFNULL(alt_nicks, ""), owner, admin
		FROM users WHERE nick=? LIMIT 1`, nick)
	if err != nil {
		return
	}
	defer result.Close()

	// Get user data.
	if result.Next() {
		var altNicksStr string
		if err = result.Scan(&dbNick, &password, &altNicksStr, &owner, &admin); err != nil {
			return
		}
		for _, altNick := range strings.Split(altNicksStr, "|") {
			altNicks[altNick] = true
		}
	}

	// Check if the nick is indeed what we want.
	if dbNick != nick {
		err = errors.New("User not in the database.")
		return
	}

	return
}

// authenticateUser authenticates the user as an owner or admin
func (bot *Bot) authenticateUser(nick, fullName, password string) error {
	_, dbPassword, _, owner, admin, err := bot.getUserData(nick)
	if err != nil {
		return errors.New(fmt.Sprintf("Error when getting user data for %s: %s", nick, err))
	}
	// Check the password
	if utils.HashPassword(password) != dbPassword {
		return errors.New("Invalid password for user")
	}
	// Check if user has any privileges
	if owner {
		bot.log.Info("Authenticating %s as an owner.", nick)
		bot.authenticatedOwners[fullName] = nick
	}
	if admin {
		bot.log.Info("Authenticating %s as an admin.", nick)
		bot.authenticatedAdmins[fullName] = nick
	}
	if !admin && !owner {
		bot.log.Info("Authenticating %s with no special privileges.", nick)
		bot.authenticatedUsers[fullName] = nick
	}
	return nil
}

// userIsMe checks if the sender is the bot.
func (bot *Bot) userIsMe(nick string) bool {
	return nick == bot.irc.OriginalName
}

// userIsAuthenticated checks if the user is authenticated with the bot.
func (bot *Bot) userIsAuthenticated(fullName string) bool {
	return bot.authenticatedOwners[fullName] != "" || bot.authenticatedAdmins[fullName] != "" ||
		bot.authenticatedUsers[fullName] != ""
}

// userIsOwner checks if the user is an authenticated owner.
func (bot *Bot) userIsOwner(fullName string) bool {
	return bot.authenticatedOwners[fullName] != ""
}

// userIsOwner checks if the user is an authenticated admin.
func (bot *Bot) userIsAdmin(fullName string) bool {
	return bot.authenticatedAdmins[fullName] != ""
}

// areSamePeople checks if two nicks belong to the same person.
func (bot *Bot) areSamePeople(nick1, nick2 string) bool {
	nick1 = strings.Trim(nick1, "_~")
	nick2 = strings.Trim(nick2, "_~")
	return nick1 == nick2
}
