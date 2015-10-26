package main

import (
	"database/sql"
)

func InitDatabase() error {
	var err error
	db, err = sql.Open("sqlite3", "papabot.db")
	if err != nil {
		return err
	}

	// Create tables
	query := `
		CREATE TABLE IF NOT EXISTS "urls" (
			"id" INTEGER PRIMARY KEY  AUTOINCREMENT  NOT NULL,
			"channel" VARCHAR NOT NULL,
			"nick" VARCHAR NOT NULL,
			"link" VARCHAR NOT NULL,
			"quote" VARCHAR NOT NULL,
			"title" VARCHAR,
			"timestamp" DATE DEFAULT (datetime('now','localtime'))
		);`
	_, err = db.Exec(query)
	if err != nil {
		return err
	}
	return nil
}
