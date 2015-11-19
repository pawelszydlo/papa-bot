package papaBot

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
)

// initDb initializes the bot's database.
func (bot *Bot) initDb() error {
	db, err := sql.Open("sqlite3", "papabot.db")
	if err != nil {
		return err
	}

	// Create URLs tables and triggers, if needed.
	query := `
		-- Main URLs table.
		CREATE TABLE IF NOT EXISTS "urls" (
			"id" INTEGER PRIMARY KEY  AUTOINCREMENT  NOT NULL,
			"channel" VARCHAR NOT NULL,
			"nick" VARCHAR NOT NULL,
			"link" VARCHAR NOT NULL,
			"quote" VARCHAR NOT NULL,
			"title" VARCHAR,
			"timestamp" DATETIME DEFAULT (datetime('now','localtime'))
		);

		-- Virtual table for FTS.
		CREATE VIRTUAL TABLE IF NOT EXISTS urls_search
		USING fts4(channel, nick, link, title, timestamp, search);

		-- Triggers for FTS updating.
		CREATE TRIGGER IF NOT EXISTS url_add AFTER INSERT ON urls BEGIN
			INSERT INTO urls_search(channel, nick, link, title, timestamp, search)
			VALUES(new.channel, new.nick, new.link, new.title, new.timestamp, new.link || ' ' || new.title);
		END;

		CREATE TRIGGER IF NOT EXISTS url_update AFTER UPDATE ON urls BEGIN
			UPDATE urls_search SET title = new.title, search = new.link || ' ' || new.title
			WHERE timestamp = new.timestamp;
		END;

		-- Users table.
		CREATE TABLE IF NOT EXISTS "users" (
			"nick" VARCHAR PRIMARY KEY NOT NULL UNIQUE,
			"password" VARCHAR,
			"alt_nicks" VARCHAR,
			"owner" boolean DEFAULT 0,
			"admin" boolean DEFAULT 0,
			"joined" DATETIME DEFAULT (datetime('now','localtime'))
		);

		-- Custom variables.
		CREATE TABLE IF NOT EXISTS "vars" (
			"name" VARCHAR PRIMARY KEY NOT NULL UNIQUE,
			"value" VARCHAR
		);
	`
	if _, err := db.Exec(query); err != nil {
		bot.Log.Panic(err)
	}

	bot.Db = db
	return nil
}
