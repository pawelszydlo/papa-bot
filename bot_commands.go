package papaBot

import (
	"fmt"
	"math/rand"
	"strings"
)

// initBotCommands registers bot commands.
func (bot *Bot) initBotCommands() {
	bot.commands = map[string]botCommand{
		"auth": {
			true, false,
			"auth password", "Authenticate with the bot.",
			bot.commandAuth},
		"reload_texts": {
			true, true,
			"reload_texts", "Reload bot's texts from file.",
			bot.commandReloadTexts},
		"find": {
			false, false,
			"find token1 token2 token3 ...", "Look for URLs containing all the tokens.",
			bot.commandFindUrl},
		"more": {
			false, false,
			"more", "Say more about last link.",
			bot.commandSayMore},
	}

	bot.commandUseLimit = map[string]int{}
}

// handleBotCommand handle command directed at the bot.
func (bot *Bot) handleBotCommand(channel, nick, user, command string) {
	receiver := channel
	// Was this command sent on a private query?
	private := false
	if bot.isMe(channel) {
		private = true
		receiver = nick
	}
	// Was the command sent by the owner?
	sender_complete := nick + "!" + user
	owner := false
	if sender_complete == bot.BotOwner {
		owner = true
	}

	params := strings.Split(command, " ")
	command = params[0]
	params = params[1:]

	bot.log.Info("Received command '%s' from '%s' on '%s' with params %s", command, nick, channel, params)

	if len(command) < 3 {
		return
	}

	if !private && !owner { // Command limits apply
		if bot.commandUseLimit[command+nick] >= bot.Config.CommandsPer5 {
			bot.SendMessage(receiver, fmt.Sprintf("%s, %s", nick, bot.Texts.CommandLimit))
			return
		} else {
			bot.commandUseLimit[command+nick] += 1
		}
	}

	// Print help
	if command == "help" {
		for _, cmd := range bot.commands {
			options := ""
			if cmd.privateOnly {
				options = " (query only)"
			}
			if cmd.ownerOnly && !owner {
				continue
			}
			bot.SendNotice(receiver, fmt.Sprintf("%s - %s%s", cmd.usage, cmd.help, options))
		}
		return
	}

	if cmd, exists := bot.commands[command]; exists {
		// Check if command needs to be run through query
		if cmd.privateOnly && !private {
			bot.SendMessage(channel, fmt.Sprintf("%s, %s", nick, bot.Texts.NeedsPriv))
			return
		}
		// Check if command needs to be run by the owner
		if cmd.ownerOnly && !owner {
			bot.SendMessage(receiver, fmt.Sprintf("%s, %s", nick, bot.Texts.NeedsOwner))
			return
		}
		cmd.commandFunc(nick, user, channel, receiver, private, params)
	} else { // Unknown command, say something
		bot.SendMessage(receiver, fmt.Sprintf(
			"%s, %s", nick, bot.Texts.WrongCommand[rand.Intn(len(bot.Texts.WrongCommand))]))
	}
}

func (bot *Bot) commandAuth(nick, user, channel, receiver string, priv bool, params []string) {
	if len(params) == 1 && HashPassword(params[0]) == bot.Config.OwnerPassword {
		bot.SendMessage(receiver, bot.Texts.PasswordOk)
		bot.BotOwner = nick + "!" + user
		bot.log.Info("Owner set to: %s", bot.BotOwner)
	}
}

func (bot *Bot) commandReloadTexts(nick, user, channel, receiver string, priv bool, params []string) {
	bot.log.Info("Reloading texts...")
	bot.loadTexts()
}

func (bot *Bot) commandSayMore(nick, user, channel, receiver string, priv bool, params []string) {
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

func (bot *Bot) commandFindUrl(nick, user, channel, receiver string, priv bool, params []string) {
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

	// Query FTS table
	result, err := bot.Db.Query(query1+query2+query3, token)
	if err != nil {
		bot.log.Warning("Can't search for URLs: %s", err)
		return
	}

	defer result.Close()

	// Announce results
	found := []string{}
	for result.Next() {
		var nick, timestr, link, title string
		if err = result.Scan(&nick, &timestr, &link, &title); err != nil {
			bot.log.Warning("Error getting search results: %s", err)
		} else {
			if priv { // skip the author and time when not on a channel
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
