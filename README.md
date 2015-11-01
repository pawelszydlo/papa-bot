# papa-bot
IRC bot written in Go. Quite comprehensive in design and IRC events handling, easily extensible.

Built on top of [github.com/nickvanw/ircx](http://github.com/nickvanw/ircx), which in turn is built on top of [github.com/sorcix/irc](http://github.com/sorcix/irc).

###How good is it?

Bot is actively developed and constantly used by me and my friends, so there's that.

###Features

* Easy configuration through a TOML file.
* Things that the bot says are all in TOML files, for easy editing and l18n.
* Flood protection.
* Abuse protection.
* Stores all the links posted on the channel.
* Allows full text search through the links.
* Logs all channel activity.

###Bundled URL processors

* Title, description and duplicates announce.
* GitHub repository information

###Bunded extensions

* BTC price command.

###TODO

* Proper Go style comments.
* Better logical separation of different components.

