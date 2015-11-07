package papaBot

// Handlers for default bot commands.

import (
	"fmt"
	"math/rand"
	"strings"
)

// initBotCommands registers bot commands.
func (bot *Bot) initBotCommands() {
	bot.commands["auth"] = &BotCommand{
		true, false, false,
		"auth password", "Authenticate with the bot.",
		commandAuth}
	bot.commands["reload_texts"] = &BotCommand{
		true, false, true,
		"reload_texts", "Reload bot's texts from file.",
		commandReloadTexts}
	bot.commands["find"] = &BotCommand{
		false, false, false,
		"find token1 token2 token3 ...", "Look for URLs containing all the tokens.",
		commandFindUrl}
	bot.commands["more"] = &BotCommand{
		false, false, false,
		"more", "Say more about last link.",
		commandSayMore}
	bot.commands["var"] = &BotCommand{
		true, true, false,
		"var list | get [name] | set [name] [value]", "Controls custom variables",
		commandVar}

	bot.commandsHideParams["auth"] = true
}

// handleBotCommand handles commands directed at the bot.
func (bot *Bot) handleBotCommand(channel, nick, user, command string) {
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
				bot.SendMessage(receiver, fmt.Sprintf("%s, %s", nick, bot.Texts.CommandLimit))
				bot.commandWarn[channel] = true
			}
			return
		} else {
			bot.commandUseLimit[command+nick] += 1
		}
	}

	// Print help.
	if command == "help" {
		for _, cmd := range bot.commands {
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
			bot.SendMessage(nick, fmt.Sprintf("%s - %s%s", cmd.HelpUsage, cmd.HelpDescription, options))
		}
		return
	}

	if cmd, exists := bot.commands[command]; exists {
		// Check if command needs to be run through query.
		if cmd.Private && !private {
			bot.SendMessage(channel, fmt.Sprintf("%s, %s", nick, bot.Texts.NeedsPriv))
			return
		}
		// Check if command needs to be run by the owner.
		if cmd.Owner && !owner || cmd.Admin && !admin {
			bot.SendMessage(receiver, fmt.Sprintf("%s, %s", nick, bot.Texts.NeedsAdmin))
			return
		}
		cmd.CommandFunc(bot, nick, user, channel, receiver, private, params)
	} else { // Unknown command, say something.
		bot.SendMessage(receiver, fmt.Sprintf(
			"%s, %s", nick, bot.Texts.WrongCommand[rand.Intn(len(bot.Texts.WrongCommand))]))
	}
}

// commandAuth is a command for authenticating an user with the bot.
func commandAuth(bot *Bot, nick, user, channel, receiver string, priv bool, params []string) {
	if len(params) == 1 {
		if err := bot.authenticateUser(nick, nick+"!"+user, params[0]); err != nil {
			bot.log.Warning("Couldn't authenticate %s: %s", nick, err)
			return
		}
		bot.SendMessage(receiver, bot.Texts.PasswordOk)
	}
}

// commandReloadTexts reloads texts from TOML file.
func commandReloadTexts(bot *Bot, nick, user, channel, receiver string, priv bool, params []string) {
	bot.log.Info("Reloading texts...")
	bot.LoadTexts(bot.textsFile, bot.Texts)
	bot.SendMessage(receiver, "Done.")
}

// commandVar gets, sets and lists custom variables.
func commandVar(bot *Bot, nick, user, channel, receiver string, priv bool, params []string) {
	if len(params) < 1 {
		return
	}
	command := params[0]
	if command == "list" {
		bot.SendMessage(receiver, "Custom variables:")
		bot.SendMessage(receiver, fmt.Sprintf("%+v", bot.customVars))
		return
	}

	if len(params) == 2 && command == "get" {
		name := params[1]
		bot.SendMessage(receiver, fmt.Sprintf("%s = %s", name, bot.getVar(name)))
		return
	}

	if len(params) > 3 && command == "set" {
		name := params[1]
		value := strings.Join(params[2:], " ")
		bot.setVar(name, value)
		bot.SendMessage(receiver, fmt.Sprintf("%s = %s", name, bot.getVar(name)))
		return
	}
}

// commandSayMore gives more info, if bot has any.
func commandSayMore(bot *Bot, nick, user, channel, receiver string, priv bool, params []string) {
	if bot.urlMoreInfo[receiver] == "" {
		bot.SendMessage(receiver, fmt.Sprintf("%s, %s", nick, bot.Texts.NothingToAdd))
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
			bot.SendMessage(receiver, bot.Texts.SearchPrivateNotice)
		}
		bot.SendMessage(receiver, fmt.Sprintf("%s, %s", nick, bot.Texts.SearchResults))
		for i := range found {
			bot.SendMessage(receiver, found[i])
		}
	} else {
		bot.SendMessage(receiver, fmt.Sprintf("%s, %s", nick, bot.Texts.SearchNoResults))
	}
}
