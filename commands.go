package papaBot

// Handlers for default bot commands.

import (
	"fmt"
	"github.com/pawelszydlo/papa-bot/events"
	"github.com/sirupsen/logrus"
	"math/rand"
	"strings"
)

// TODO: universal way of handling formatting throughout transports.
// initBotCommands registers bot commands.
func (bot *Bot) initBotCommands() {
	// Help.
	bot.RegisterCommand(&BotCommand{
		[]string{"help", "h"},
		false, false, false,
		"[pub]", "Send help text to you privately. Adding [pub] will print help on the same channel you asked.",
		commandHelp})
	// Auth.
	bot.RegisterCommand(&BotCommand{
		[]string{"auth"},
		true, false, false,
		"<username> <password>", "Authenticate with the bot.",
		commandAuth})
	// Useradd.
	bot.RegisterCommand(&BotCommand{
		[]string{"useradd"},
		true, false, false,
		"<username> <password>", "Create user account.",
		commandUserAdd})
	// Find.
	bot.RegisterCommand(&BotCommand{
		[]string{"f", "find"},
		false, false, false,
		"<token1> <token2> <token3> ...", "Look for URLs containing all the tokens.",
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
		"list | get <name> | set <name> <value>", "Controls custom variables.",
		commandVar})
	// Ignore.
	bot.RegisterCommand(&BotCommand{
		[]string{"ignore"},
		false, true, true,
		"add <nick> <userName> | remove <nick> <userName>", "Manages ignore list.",
		commandIgnore})

	bot.commandsHideParams["auth"] = true
	bot.commandsHideParams["useradd"] = true
}

// handleBotCommand handles commands directed at the bot.
func (bot *Bot) handleBotCommand(message events.EventMessage) {
	// Catch errors.
	defer func() {
		if Debug {
			return
		} // When in debug mode fail on all errors.
		if r := recover(); r != nil {
			bot.Log.Errorf("FATAL ERROR in bot command: %s", r)
		}
	}()

	// Was this command sent through a private message?
	private := false
	if message.EventCode == events.EventPrivateMessage {
		private = true
	}
	// Was the command sent by the owner?
	owner := bot.UserIsOwner(message.FullName)
	admin := bot.UserIsAdmin(message.FullName)

	params := strings.Split(message.Message, " ")
	command := params[0]
	params = params[1:]

	paramsDisplay := fmt.Sprintf("%+v", params)
	if bot.commandsHideParams[command] {
		paramsDisplay = "<hidden>"
	}
	bot.Log.WithFields(
		logrus.Fields{"channel": message.Channel, "cmd": command, "params": paramsDisplay},
	).Infof("Received command from %s.", message.Nick)

	if !private && !owner && !admin { // Command limits apply.
		if bot.commandUseLimit[command+message.Nick] >= bot.Config.CommandsPer5 { // Per command+person.
			if !bot.commandWarn[message.Channel] { // Warning was not yet sent.
				bot.SendMessage(
					message.SourceTransport, message.Channel, fmt.Sprintf("%s, %s", message.Nick, bot.Texts.CommandLimit))
				bot.commandWarn[message.Channel] = true
			}
			return
		} else {
			bot.commandUseLimit[command+message.Nick] += 1
		}
	}

	if cmd, exists := bot.commands[command]; exists {
		// Check if command needs to be run through private message.
		if cmd.Private && !private {
			bot.SendMessage(
				message.SourceTransport, message.Channel, fmt.Sprintf("%s, %s", message.Nick, bot.Texts.NeedsPriv))
			return
		}
		// Check if command needs to be run by the owner.
		if cmd.Owner && !owner || cmd.Admin && !admin {
			bot.SendAutoMessage(
				private,
				message.SourceTransport, message.Nick, message.Channel,
				fmt.Sprintf("%s, %s", message.Nick, bot.Texts.NeedsAdmin))
			return
		}
		cmd.CommandFunc(
			bot, message.Nick, message.FullName, message.Channel, message.SourceTransport, private, params)
	} else { // Unknown command.
		if rand.Int()%10 > 3 {
			bot.SendAutoMessage(private, message.SourceTransport, message.Nick, message.Channel, fmt.Sprintf(
				"%s", bot.Texts.WrongCommand[rand.Intn(len(bot.Texts.WrongCommand))]))

		}
	}
}

// commandHelp will print help for all the commands.
func commandHelp(bot *Bot, nick, user, channel, transport string, priv bool, params []string) {
	if len(params) == 0 || params[0] != "pub" { // By default help only gets sent on priv.
		priv = true
	}

	owner := bot.UserIsOwner(user)
	admin := bot.UserIsAdmin(user)
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
			if transport == "irc" {
				options = " \x0300(private only)\x03"
			} else {
				options = " (private only)"
			}
		}
		if cmd.Owner && !owner {
			continue
		}
		if cmd.Admin && !admin {
			continue
		}
		message := ""
		if transport == "irc" {
			message = fmt.Sprintf(
				"\x0308%s\x03 \x0310%s\x03 - %s%s", commands, cmd.HelpParams, cmd.HelpDescription, options)
		} else {
			message = fmt.Sprintf("%s %s - %s%s", commands, cmd.HelpParams, cmd.HelpDescription, options)
		}
		bot.SendAutoMessage(priv, transport, nick, channel, message)

	}
	return
}

// commandAuth is a command for authenticating an user with the bot.
func commandAuth(bot *Bot, nick, user, channel, transport string, priv bool, params []string) {
	if len(params) == 2 {
		if err := bot.authenticateUser(params[0], user, params[1]); err != nil {
			bot.Log.Warningf("Couldn't authenticate %s: %s", nick, err)
			return
		}
		bot.SendPrivMessage(transport, nick, "You are now logged in.")
	}
}

// commandIgnore will control the ignore list.
func commandIgnore(bot *Bot, nick, user, channel, transport string, priv bool, params []string) {
	if len(params) == 2 {
		command := params[0]
		fullName := params[1]
		if command == "add" {
			if bot.UserIsOwner(user) {
				bot.SendPrivMessage(transport, nick, "You cannot ignore the owner.")
				return
			}
			bot.AddToIgnoreList(fullName)
		} else if command == "remove" {
			bot.RemoveFromIgnoreList(fullName)
		}
		bot.SendPrivMessage(transport, nick, "Ignore list changed.")
	}
}

// commandUserAdd will add a new user to bot's database and authenticate.
func commandUserAdd(bot *Bot, nick, user, channel, transport string, priv bool, params []string) {
	if len(params) == 2 {
		if bot.UserIsAuthenticated(user) {
			bot.SendPrivMessage(transport, nick, "You are already authenticated.")
			return
		}

		if err := bot.addUser(params[0], params[1], false, false); err != nil {
			bot.Log.Warningf("Couldn't add user %s: %s", params[0], err)
			bot.SendPrivMessage(transport, nick, fmt.Sprintf("Can't add user: %s", err))
			return
		}
		if err := bot.authenticateUser(params[0], nick+"!"+user, params[1]); err != nil {
			bot.Log.Warningf("Couldn't authenticate %s: %s", nick, err)
			return
		}
		bot.SendPrivMessage(transport, nick, "User added. You are now logged in.")
	}
}

// commandVar gets, sets and lists custom variables.
func commandVar(bot *Bot, nick, user, channel, transport string, priv bool, params []string) {
	if len(params) < 1 {
		return
	}
	command := params[0]
	if command == "list" {
		bot.SendPrivMessage(transport, nick, "Custom variables:")
		for key, val := range bot.customVars {
			bot.SendPrivMessage(transport, nick, fmt.Sprintf("%s = %s", key, val))
		}
		return
	}

	if len(params) == 2 && command == "get" {
		name := params[1]
		bot.SendPrivMessage(transport, nick, fmt.Sprintf("%s = %s", name, bot.GetVar(name)))
		return
	}

	if len(params) >= 3 && command == "set" {
		name := params[1]
		value := strings.Join(params[2:], " ")
		bot.SetVar(name, value)
		bot.SendPrivMessage(transport, nick, fmt.Sprintf("%s = %s", name, bot.GetVar(name)))
		return
	}
}

// commandSayMore gives more info, if bot has any.
func commandSayMore(bot *Bot, nick, user, channel, transport string, priv bool, params []string) {

	if bot.urlMoreInfo[transport+channel] == "" {
		bot.SendAutoMessage(priv, transport, nick, channel, fmt.Sprintf("%s, %s", nick, bot.Texts.NothingToAdd))
		return
	} else {
		bot.SendAutoMessage(priv, transport, nick, channel, bot.urlMoreInfo[transport+channel])
		delete(bot.urlMoreInfo, transport+channel)
	}
}

// commandFindUrl searches bot's database using FTS for links matching the query.
func commandFindUrl(bot *Bot, nick, user, channel, transport string, priv bool, params []string) {
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
		bot.Log.Warningf("Can't search for URLs: %s", err)
		return
	}

	defer result.Close()

	// Announce results.
	found := []string{}
	for result.Next() {
		var nick, timestr, link, title string
		if err = result.Scan(&nick, &timestr, &link, &title); err != nil {
			bot.Log.Warningf("Error getting search results: %s", err)
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
			bot.SendPrivMessage(transport, nick, bot.Texts.SearchPrivateNotice)
		}
		bot.SendAutoMessage(priv, transport, nick, channel, fmt.Sprintf("%s, %s", nick, bot.Texts.SearchResults))
		for i := range found {
			bot.SendAutoMessage(priv, transport, nick, channel, found[i])
		}
	} else {
		bot.SendAutoMessage(priv, transport, nick, channel, fmt.Sprintf("%s", bot.Texts.SearchNoResults))
	}
}
