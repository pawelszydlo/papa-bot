# papa-bot
IRC bot written in Go. Quite comprehensive in design and IRC events handling, easily extensible.

Built on top of [github.com/nickvanw/ircx](http://github.com/nickvanw/ircx), which in turn is built on top of [github.com/sorcix/irc](http://github.com/sorcix/irc).

###How good is it?

Bot is actively developed and constantly used by me and my friends, so there's that.

###Features

* Easy to write extensions.
* Easy configuration through a TOML file.
* Things that the bot says are all in TOML files, for easy editing and l18n.
* Flood protection.
* Abuse protection.
* Stores all the links posted on the channel.
* Allows full text search through the links.
* Logs all channel activity.

###Bundled extensions

* Title, description and duplicates announce.
* GitHub repository information.
* BTC price command.

###Adding your own extensions

To add your own extension you just need to create a struct that will implement these methods:
```go
// Will be run on bot init.
func (ext *YourExtension) Init(bot *Bot) {
    // Extension initialization goes here.
}

// Will be run for each URL found in the message.
func (ext *YourExtension) ProcessURL(bot *Bot, urlinfo *UrlInfo, channel, sender, msg string) {
    // Here you can do something with the data in the urlinfo struct.
}
```

Then add your extension to the list in the bot's Run function:
```go
extensions: []Extension{
    new(ExtensionMeta),
    new(ExtensionGitHub),
    new(ExtensionBtc),
    // Your extension goes here.
},
```

If you don't want to implement any of those just leave them empty.

###TODO

* Proper Go style comments.

