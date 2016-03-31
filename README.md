# papa-bot
IRC bot written in Go. Comprehensive in design and IRC events handling, easily extensible.

Built using [github.com/sorcix/irc](http://github.com/sorcix/irc) IRC library.

[Full documentation](https://godoc.org/github.com/pawelszydlo/papa-bot) @ GoDoc.org

###How good is it?

Bot is actively developed and constantly used by me and my friends, so there's that.

###Features

* Easy to write extensions.
* Easy configuration through a TOML file and run time variables.
* Things that the bot says are all in TOML files, for easy editing and l18n.
* Flood protection.
* Abuse protection.
* Stores all the links posted on the channel.
* Allows full text search through the links.
* Logs all channel activity.
* User accounts and permissions handling.

###Bundled extensions

* Link title and description.
* Link duplicates announce.
* Check if link was posted on Reddit.
* GitHub repository information.
* BTC price command.
* Movie information.
* Custom counters/countdowns.
* Raw IRC commands.
* Wikipedia article lookup.
* Wolfram Alpha lookup.

###Adding your own extensions

Very simple, just take a look [at the example](https://github.com/pawelszydlo/papa-bot/blob/master/example/example.go). 

####TODO

* Split handling.
* Alt nicks handling.
* Make extensions work based on events.