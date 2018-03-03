# papa-bot
Bot written in Go. Comprehensive in design, easily extensible.
Initially designed for IRC, now supports multiple transports.

[Full documentation](https://godoc.org/github.com/pawelszydlo/papa-bot) @ GoDoc.org

### How good is it?

Bot is actively developed and constantly used by me and my friends, so there's that.

### Features

* Multiple transports support.
* Easy to write extensions (just take a look [at the example](https://github.com/pawelszydlo/papa-bot/blob/master/example/example.go))
* Event based operation.
* Configuration through a TOML file and persistent run time variables.
* All text messages are in TOML files, for easy editing and l18n.
* Flood protection.
* Abuse protection.
* Ignore list.
* Stores all the links posted on the channel.
* Allows full text search through the links.
* Logs all channel activity.
* User accounts and permissions handling.

### Supported transports

* IRC (built using [github.com/sorcix/irc](http://github.com/sorcix/irc) library.)
* that's about it for now...

### Bundled extensions

* Link title and description.
* Link duplicates announce.
* Check if link was posted on Reddit.
* GitHub repository information.
* BTC price command.
* Movie information.
* Custom counters/countdowns.
* Wikipedia article lookup.
* Wolfram Alpha lookup.


#### TODO

Urgent:
* Write some tests!
* Refactor extensions to format replies according to transport.

Wishlist:
* IRC split handling.
* Alt nicks handling, cross-transport people identification.
