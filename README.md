# papa-bot
IRC bot written in Go. Quite comprehensive in design and IRC events handling, easily extendable.

Built on top of [github.com/nickvanw/ircx](http://github.com/nickvanw/ircx), which in turn is built on top of [github.com/sorcix/irc](http://github.com/sorcix/irc).

###How good is it?

It is my first project written in Go, so the implementation might be lacking. That being said, bot is actively developed
and used by me and my friends, so there's that.

###Features

* Easy configuration through a TOML file.
* Things that the bot says are all in one TOML file, for easy editing and l18n.
* Flood protection.
* Abuse protection.
* Stores all the links posted on the channel, with title.
* Provides additional information on links: duplicate postings, description, GitHub statistics.
* Allows full text search through the links.
* Logs all channel activity.

###TODO

* A lot...

