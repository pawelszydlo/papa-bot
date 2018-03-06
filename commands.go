package papaBot

// Handlers for default bot commands.

import (
	"fmt"
	"github.com/pawelszydlo/papa-bot/events"
	"github.com/sirupsen/logrus"
	"math/rand"
	"strings"
)

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
		"add <userName> | remove <userName>", "Manages ignore list.",
		commandIgnore})

	bot.commandsHideParams["auth"] = true
	bot.commandsHideParams["useradd"] = true
}

// handleBotCommand handles commands directed at the bot.
func (bot *Bot) handleBotCommand(sourceEvent *events.EventMessage) {
	// Catch errors.
	defer func() {
		// Run a work done event.
		bot.EventDispatcher.Trigger(events.EventMessage{
			sourceEvent.TransportName,
			sourceEvent.TransportFormatting,
			events.EventBotDone,
			"", "", sourceEvent.Channel, "", sourceEvent.Context, false,
		})
		if Debug {
			return
		} // When in debug mode fail on all errors.
		if r := recover(); r != nil {
			bot.Log.Errorf("FATAL ERROR in bot command: %s", r)
		}
	}()

	// Run a work start event.
	bot.EventDispatcher.Trigger(events.EventMessage{
		sourceEvent.TransportName,
		sourceEvent.TransportFormatting,
		events.EventBotWorking,
		"", "", sourceEvent.Channel, "", "", false,
	})

	// Was the command sent by the owner?
	owner := bot.UserIsOwner(sourceEvent.UserId)
	admin := bot.UserIsAdmin(sourceEvent.UserId)

	params := strings.Split(sourceEvent.Message, " ")
	command := params[0]
	params = params[1:]

	paramsDisplay := fmt.Sprintf("%+v", params)
	if bot.commandsHideParams[command] {
		paramsDisplay = "<hidden>"
	}
	bot.Log.WithFields(
		logrus.Fields{"channel": sourceEvent.Channel, "cmd": command, "params": paramsDisplay},
	).Infof("Received command from %s.", sourceEvent.Nick)

	if !sourceEvent.IsPrivate() && !owner && !admin { // Command limits apply.
		if bot.commandUseLimit[command+sourceEvent.Nick] >= bot.Config.CommandsPer5 { // Per command+person.
			if !bot.commandWarn[sourceEvent.Channel] { // Warning was not yet sent.
				bot.SendMessage(sourceEvent, fmt.Sprintf("%s, %s", sourceEvent.Nick, bot.Texts.CommandLimit))
				bot.commandWarn[sourceEvent.Channel] = true
			}
			return
		} else {
			bot.commandUseLimit[command+sourceEvent.Nick] += 1
		}
	}

	if cmd, exists := bot.commands[command]; exists {
		// Check if command needs to be run through private message.
		if cmd.Private && !sourceEvent.IsPrivate() {
			bot.SendMessage(sourceEvent, fmt.Sprintf("%s, %s", sourceEvent.Nick, bot.Texts.NeedsPriv))
			return
		}
		// Check if command needs to be run by the owner.
		if cmd.Owner && !owner || cmd.Admin && !admin {
			bot.SendMessage(sourceEvent, fmt.Sprintf("%s, %s", sourceEvent.Nick, bot.Texts.NeedsAdmin))
			return
		}
		// Execute the command.
		cmd.CommandFunc(bot, sourceEvent, params)
	} else { // Unknown command.
		if rand.Int()%10 > 3 {
			bot.SendMessage(sourceEvent, fmt.Sprintf("%s", bot.Texts.WrongCommand[rand.Intn(len(bot.Texts.WrongCommand))]))

		}
	}
}

// commandHelp will print help for all the commands.
func commandHelp(bot *Bot, sourceEvent *events.EventMessage, params []string) {
	forcePriv := false
	if len(params) == 0 || params[0] != "pub" { // By default help only gets sent on priv.
		forcePriv = true
	}

	owner := bot.UserIsOwner(sourceEvent.UserId)
	admin := bot.UserIsAdmin(sourceEvent.UserId)
	// Build a list of all command aliases.
	helpCommandKeys := map[string][]string{}
	helpCommands := map[string]*BotCommand{}
	for key, cmd := range bot.commands {
		pointerStr := fmt.Sprintf("%p", cmd)
		helpCommandKeys[pointerStr] = append(helpCommandKeys[pointerStr], key)
		helpCommands[pointerStr] = cmd
	}
	// Print help.
	results := []string{}
	for pointerStr, cmd := range helpCommands {
		commands := strings.Join(helpCommandKeys[pointerStr], ", ")
		options := ""
		if cmd.Private {
			if sourceEvent.TransportFormatting == events.FormatIRC {
				options = " \x0300(private only)\x03"
			} else if sourceEvent.TransportFormatting == events.FormatMarkdown {
				options = " **(private only)**"
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
		result := ""
		if sourceEvent.TransportFormatting == events.FormatIRC {
			result = fmt.Sprintf(
				"\x0308%s\x03 \x0310%s\x03 - %s%s", commands, cmd.HelpParams, cmd.HelpDescription, options)
		} else if sourceEvent.TransportFormatting == events.FormatMarkdown {
			result = fmt.Sprintf("| %s | %s | %s%s |", commands, cmd.HelpParams, cmd.HelpDescription, options)
		} else {
			result = fmt.Sprintf("%s %s - %s%s", commands, cmd.HelpParams, cmd.HelpDescription, options)
		}
		results = append(results, result)
	}
	// Send the help messages.
	if sourceEvent.TransportFormatting == events.FormatMarkdown {
		result := "\n\n| Command | Parameters | Help |\n| :-- | :-- | :-- |\n"
		result += strings.Join(results, "\n")
		if forcePriv {
			bot.SendPrivateMessage(sourceEvent, sourceEvent.Nick, result)
		} else {
			bot.SendMessage(sourceEvent, result)
		}
	} else {
		for _, result := range results {
			if forcePriv {
				bot.SendPrivateMessage(sourceEvent, sourceEvent.Nick, result)
			} else {
				bot.SendMessage(sourceEvent, result)
			}
		}
	}


	return
}

// commandAuth is a command for authenticating an user with the bot.
func commandAuth(bot *Bot, sourceEvent *events.EventMessage, params []string) {
	if len(params) == 2 {
		if err := bot.authenticateUser(params[0], sourceEvent.UserId, params[1]); err != nil {
			bot.Log.Warningf("Couldn't authenticate %s: %s", params[0], err)
			return
		}
		bot.SendMessage(sourceEvent, "You are now logged in.")
	}
}

// commandIgnore will control the ignore list.
func commandIgnore(bot *Bot, sourceEvent *events.EventMessage, params []string) {
	if len(params) == 2 {
		command := params[0]
		userId := params[1]
		if command == "add" {
			if bot.UserIsOwner(userId) {
				bot.SendMessage(sourceEvent, "You cannot ignore the owner.")
				return
			}
			bot.AddToIgnoreList(userId)
		} else if command == "remove" {
			bot.RemoveFromIgnoreList(userId)
		}
		bot.SendMessage(sourceEvent, "Ignore list changed.")
	}
}

// commandUserAdd will add a new user to bot's database and authenticate.
func commandUserAdd(bot *Bot, sourceEvent *events.EventMessage, params []string) {
	if len(params) == 2 {
		if bot.UserIsAuthenticated(sourceEvent.UserId) {
			bot.SendMessage(sourceEvent, "You are already authenticated.")
			return
		}

		if err := bot.addUser(params[0], params[1], false, false); err != nil {
			bot.Log.Warningf("Couldn't add user %s: %s", params[0], err)
			bot.SendMessage(sourceEvent, fmt.Sprintf("Can't add user: %s", err))
			return
		}
		if err := bot.authenticateUser(params[0], sourceEvent.UserId, params[1]); err != nil {
			bot.Log.Warningf("Couldn't authenticate %s: %s", params[0], err)
			return
		}
		bot.SendMessage(sourceEvent, "User added. You are now logged in.")
	}
}

// commandVar gets, sets and lists custom variables.
func commandVar(bot *Bot, sourceEvent *events.EventMessage, params []string) {
	if len(params) < 1 {
		return
	}
	command := params[0]
	if command == "list" {
		bot.SendMessage(sourceEvent, "Custom variables:")
		for key, val := range bot.customVars {
			bot.SendMessage(sourceEvent, fmt.Sprintf("%s = %s", key, val))
		}
		return
	}

	if len(params) == 2 && command == "get" {
		name := params[1]
		bot.SendMessage(sourceEvent, fmt.Sprintf("%s = %s", name, bot.GetVar(name)))
		return
	}

	if len(params) >= 3 && command == "set" {
		name := params[1]
		value := strings.Join(params[2:], " ")
		bot.SetVar(name, value)
		bot.SendMessage(sourceEvent, fmt.Sprintf("%s = %s", name, bot.GetVar(name)))
		return
	}
}

// commandSayMore gives more info, if bot has any.
func commandSayMore(bot *Bot, sourceEvent *events.EventMessage, params []string) {

	if bot.urlMoreInfo[sourceEvent.TransportName+sourceEvent.Channel] == "" {
		bot.SendMessage(sourceEvent, fmt.Sprintf("%s, %s", sourceEvent.Nick, bot.Texts.NothingToAdd))
		return
	} else {
		bot.SendMessage(sourceEvent, bot.urlMoreInfo[sourceEvent.TransportName+sourceEvent.Channel])
		delete(bot.urlMoreInfo, sourceEvent.TransportName+sourceEvent.Channel)
	}
}

// commandFindUrl searches bot's database using FTS for links matching the query.
func commandFindUrl(bot *Bot, sourceEvent *events.EventMessage, params []string) {
	if len(params) == 0 {
		return
	}
	token := strings.Join(params, " AND ")

	query1 := "SELECT nick, timestamp, link, title FROM urls_search WHERE "
	query2 := ""
	if !sourceEvent.IsPrivate() {
		query2 = fmt.Sprintf("channel=\"%s\" AND ", sourceEvent.Channel)
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
			if sourceEvent.IsPrivate() { // skip the author and time when not on a channel.
				found = append(found, fmt.Sprintf("%s (%s)", link, title))
			} else {
				found = append(found, fmt.Sprintf("%s | %s | %s (%s)", nick, timestr, link, title))
			}
		}
	}
	if len(found) > 0 {
		if sourceEvent.IsPrivate() {
			bot.SendMessage(sourceEvent, bot.Texts.SearchPrivateNotice)
		}
		bot.SendMessage(sourceEvent, fmt.Sprintf("%s, %s", sourceEvent.Nick, bot.Texts.SearchResults))
		for i := range found {
			bot.SendMessage(sourceEvent, found[i])
		}
	} else {
		bot.SendMessage(sourceEvent, fmt.Sprintf("%s", bot.Texts.SearchNoResults))
	}
}
