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

###Adding your own extensions

To add your own extension you just need to create a struct that will embed the Extension struct:
```go
type YourExtension struct {
    Extension
}
```

Then you can override the methods you need.
```go
// Will be run on bot's init.
func (ext *YourExtension) Init(bot *Bot) error { return nil }

// Will be run whenever an URL is found in the message.
func (ext *YourExtension) ProcessURL(bot *Bot, info *UrlInfo, channel, sender, msg string) {}

// Will be run on every public message the bot receives.
func (ext *YourExtension) ProcessMessage(bot *Bot, channel, nick, msg string) {}

// Will be run every 5 minutes. Daily will be set to true once per day.
func (ext *YourExtension) Tick(bot *Bot, daily bool) {}
```
(You don't need to implement all of them.)

To enable your extension, add it to the list in the bot's Run function:
```go
extensions: []ExtensionInterface{
    new(ExtensionMeta),
    new(ExtensionGitHub),
    new(ExtensionBtc),
    // Your extension goes here.
},
```

####TODO

* Split handling.
* Alt nicks handling.