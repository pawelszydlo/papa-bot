package main

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
)

var (
	db		 *sql.DB
)

func init()  {
	if db, err = sql.Open("sqlite3", "papabot.db"); err != nil {
		lerror.Fatal("Can't open database:", err)
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
	if _, err := db.Exec(query); err != nil {
		lerror.Fatal("Can't create urls table:", err)
	}
}
