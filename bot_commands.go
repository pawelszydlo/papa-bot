package papaBot

// Handlers for default bot commands.

import (
	"fmt"
	"math/rand"
	"strings"
)

// initBotCommands registers bot commands.
func (bot *Bot) initBotCommands() {
	// Help.
	bot.RegisterCommand(&BotCommand{
		[]string{"help", "h"},
		false, false, false,
		"", "Print commands help.",
		commandHelp})
	// Auth.
	bot.RegisterCommand(&BotCommand{
		[]string{"auth"},
		true, false, false,
		"[user] [password]", "Authenticate with the bot.",
		commandAuth})
	// Useradd.
	bot.RegisterCommand(&BotCommand{
		[]string{"useradd"},
		true, false, false,
		"[user] [password]", "Create user account.",
		commandUserAdd})
	// Reload.
	bot.RegisterCommand(&BotCommand{
		[]string{"reload"},
		true, false, true,
		"", "Reload bot's texts from file.",
		commandReloadTexts})
	// Find.
	bot.RegisterCommand(&BotCommand{
		[]string{"f", "find"},
		false, false, false,
		"token1 token2 token3 ...", "Look for URLs containing all the tokens.",
		commandFindUrl})
	// More.
	bot.RegisterCommand(&BotCommand{
		[]string{"m", "more", "moar"},
		false, false, false,
		"", "Say more about last link.",
		commandSayMore})
	// Var.
	bot.RegisterCommand(&BotCommand{
		[]string{"var", "v"},
		true, true, false,
		"list | get [name] | set [name] [value]", "Controls custom variables",
		commandVar})

	bot.commandsHideParams["auth"] = true
	bot.commandsHideParams["useradd"] = true
}

// handleBotCommand handles commands directed at the bot.
func (bot *Bot) handleBotCommand(channel, nick, user, command string, talkBack bool) {
	// Catch errors.
	defer func() {
		if Debug {
			return
		} // When in debug mode fail on all errors.
		if r := recover(); r != nil {
			bot.log.Error("FATAL ERROR in bot command: %s", r)
		}
	}()
	receiver := channel

	// Was this command sent on a private query?
	private := false
	if bot.userIsMe(channel) {
		private = true
		receiver = nick
	}
	// Was the command sent by the owner?
	sender_complete := nick + "!" + user
	owner := bot.userIsOwner(sender_complete)
	admin := bot.userIsAdmin(sender_complete)

	params := strings.Split(command, " ")
	command = params[0]
	params = params[1:]

	if bot.commandsHideParams[command] {
		bot.log.Info("Received command '%s' from '%s' on '%s' with params <params hidden>.", command, nick, channel)
	} else {
		bot.log.Info("Received command '%s' from '%s' on '%s' with params %s.", command, nick, channel, params)
	}

	if len(command) < 3 {
		return
	}

	if !private && !owner && !admin { // Command limits apply.
		if bot.commandUseLimit[command+nick] >= bot.Config.CommandsPer5 {
			if !bot.commandWarn[channel] { // Warning was not yet sent.
				bot.SendPrivMessage(receiver, fmt.Sprintf("%s, %s", nick, bot.Texts.CommandLimit))
				bot.commandWarn[channel] = true
			}
			return
		} else {
			bot.commandUseLimit[command+nick] += 1
		}
	}

	if cmd, exists := bot.commands[command]; exists {
		// Check if command needs to be run through query.
		if cmd.Private && !private {
			bot.SendPrivMessage(channel, fmt.Sprintf("%s, %s", nick, bot.Texts.NeedsPriv))
			return
		}
		// Check if command needs to be run by the owner.
		if cmd.Owner && !owner || cmd.Admin && !admin {
			bot.SendPrivMessage(receiver, fmt.Sprintf("%s, %s", nick, bot.Texts.NeedsAdmin))
			return
		}
		cmd.CommandFunc(bot, nick, user, channel, receiver, private, params)
	} else { // Unknown command.
		if talkBack {
			bot.SendPrivMessage(receiver, fmt.Sprintf(
				"%s, %s", nick, bot.Texts.WrongCommand[rand.Intn(len(bot.Texts.WrongCommand))]))
		}
	}
}

// commandHelp will print help for all the commands.
func commandHelp(bot *Bot, nick, user, channel, receiver string, priv bool, params []string) {
	sender_complete := nick + "!" + user
	owner := bot.userIsOwner(sender_complete)
	admin := bot.userIsAdmin(sender_complete)
	// Build a list of all command aliases.
	helpCommandKeys := map[string][]string{}
	helpCommands := map[string]*BotCommand{}
	for key, cmd := range bot.commands {
		pointerStr := fmt.Sprintf("%p", cmd)
		helpCommandKeys[pointerStr] = append(helpCommandKeys[pointerStr], key)
		helpCommands[pointerStr] = cmd
	}
	// Print help.
	for pointerStr, cmd := range helpCommands {
		commands := strings.Join(helpCommandKeys[pointerStr], ", ")
		options := ""
		if cmd.Private {
			options = " (query only)"
		}
		if cmd.Owner && !owner {
			continue
		}
		if cmd.Admin && !admin {
			continue
		}
		bot.SendPrivMessage(
			nick, fmt.Sprintf("\x0308%s\x03 \x0310%s\x03 - %s%s", commands, cmd.HelpParams, cmd.HelpDescription, options))
	}
	return
}

// commandAuth is a command for authenticating an user with the bot.
func commandAuth(bot *Bot, nick, user, channel, receiver string, priv bool, params []string) {
	if len(params) == 2 {
		if err := bot.authenticateUser(params[0], nick+"!"+user, params[1]); err != nil {
			bot.log.Warning("Couldn't authenticate %s: %s", nick, err)
			return
		}
		bot.SendPrivMessage(receiver, "You are now logged in.")
	}
}

// commandUserAdd will add a new user to bot's database and authenticate.
func commandUserAdd(bot *Bot, nick, user, channel, receiver string, priv bool, params []string) {
	if len(params) == 2 {
		if bot.userIsAuthenticated(nick + "!" + user) {
			bot.SendPrivMessage(receiver, "You are already authenticated.")
			return
		}

		if err := bot.addUser(params[0], params[1], false, false); err != nil {
			bot.log.Warning("Couldn't add user %s: %s", params[0], err)
			bot.SendPrivMessage(receiver, fmt.Sprintf("Can't add user: %s", err))
			return
		}
		if err := bot.authenticateUser(params[0], nick+"!"+user, params[1]); err != nil {
			bot.log.Warning("Couldn't authenticate %s: %s", nick, err)
			return
		}
		bot.SendPrivMessage(receiver, "User added. You are now logged in.")
	}
}

// commandReloadTexts reloads texts from TOML file.
func commandReloadTexts(bot *Bot, nick, user, channel, receiver string, priv bool, params []string) {
	bot.log.Info("Reloading texts...")
	bot.LoadTexts(bot.textsFile, bot.Texts)
	bot.SendPrivMessage(receiver, "Done.")
}

// commandVar gets, sets and lists custom variables.
func commandVar(bot *Bot, nick, user, channel, receiver string, priv bool, params []string) {
	if len(params) < 1 {
		return
	}
	command := params[0]
	if command == "list" {
		bot.SendPrivMessage(receiver, "Custom variables:")
		bot.SendPrivMessage(receiver, fmt.Sprintf("%+v", bot.customVars))
		return
	}

	if len(params) == 2 && command == "get" {
		name := params[1]
		bot.SendPrivMessage(receiver, fmt.Sprintf("%s = %s", name, bot.GetVar(name)))
		return
	}

	if len(params) > 3 && command == "set" {
		name := params[1]
		value := strings.Join(params[2:], " ")
		bot.SetVar(name, value)
		bot.SendPrivMessage(receiver, fmt.Sprintf("%s = %s", name, bot.GetVar(name)))
		return
	}
}

// commandSayMore gives more info, if bot has any.
func commandSayMore(bot *Bot, nick, user, channel, receiver string, priv bool, params []string) {
	if bot.urlMoreInfo[receiver] == "" {
		bot.SendPrivMessage(receiver, fmt.Sprintf("%s, %s", nick, bot.Texts.NothingToAdd))
		return
	} else {
		if len(bot.urlMoreInfo[receiver]) > 500 {
			bot.urlMoreInfo[receiver] = bot.urlMoreInfo[receiver][:500]
		}
		bot.SendNotice(receiver, bot.urlMoreInfo[receiver])
		delete(bot.urlMoreInfo, receiver)
	}
}

// commandFindUrl searches bot's database using FTS for links matching the query.
func commandFindUrl(bot *Bot, nick, user, channel, receiver string, priv bool, params []string) {
	if len(params) == 0 {
		return
	}
	token := strings.Join(params, " AND ")

	query1 := "SELECT nick, timestamp, link, title FROM urls_search WHERE "
	query2 := ""
	if !priv {
		query2 = fmt.Sprintf("channel=\"%s\" AND ", channel)
	}
	query3 := "search MATCH ? GROUP BY link ORDER BY timestamp DESC LIMIT 5"

	// Query FTS table.
	result, err := bot.Db.Query(query1+query2+query3, token)
	if err != nil {
		bot.log.Warning("Can't search for URLs: %s", err)
		return
	}

	defer result.Close()

	// Announce results.
	found := []string{}
	for result.Next() {
		var nick, timestr, link, title string
		if err = result.Scan(&nick, &timestr, &link, &title); err != nil {
			bot.log.Warning("Error getting search results: %s", err)
		} else {
			if priv { // skip the author and time when not on a channel.
				found = append(found, fmt.Sprintf("%s (%s)", link, title))
			} else {
				found = append(found, fmt.Sprintf("%s | %s | %s (%s)", nick, timestr, link, title))
			}
		}
	}
	if len(found) > 0 {
		if priv {
			bot.SendPrivMessage(receiver, bot.Texts.SearchPrivateNotice)
		}
		bot.SendPrivMessage(receiver, fmt.Sprintf("%s, %s", nick, bot.Texts.SearchResults))
		for i := range found {
			bot.SendPrivMessage(receiver, found[i])
		}
	} else {
		bot.SendPrivMessage(receiver, fmt.Sprintf("%s, %s", nick, bot.Texts.SearchNoResults))
	}
}
