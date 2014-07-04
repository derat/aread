package main

import (
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"os"
)

type Database struct {
	db *sql.DB
}

func NewDatabase(path string) (*Database, error) {
	init := false
	if _, err := os.Stat(path); os.IsNotExist(err) {
		init = true
	}

	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	if init {
		sql := `
		CREATE TABLE Pages (
			Id STRING PRIMARY KEY NOT NULL,
			OriginalUrl STRING NOT NULL,
			Title STRING NOT NULL,
			TimeAdded INTEGER NOT NULL)
			`
		if _, err = db.Exec(sql); err != nil {
			return nil, fmt.Errorf("Unable to initialize database: %v", err)
		}
	}

	d := &Database{db: db}
	return d, nil
}

func (d *Database) AddPage(pi PageInfo) error {
	stmt, err := d.db.Prepare("INSERT OR REPLACE INTO Pages (Id, OriginalUrl, Title, TimeAdded) VALUES(?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()
	if _, err := stmt.Exec(pi.Id, pi.OriginalUrl, pi.Title, pi.TimeAdded); err != nil {
		return err
	}
	return nil
}

func (d *Database) GetPages() (pages []PageInfo, err error) {
	rows, err := d.db.Query("SELECT Id, OriginalUrl, Title, TimeAdded FROM Pages ORDER BY TimeAdded DESC")
	if err != nil {
		return pages, err
	}
	defer rows.Close()
	for rows.Next() {
		pi := PageInfo{}
		rows.Scan(&pi.Id, &pi.OriginalUrl, &pi.Title, &pi.TimeAdded)
		pages = append(pages, pi)
	}
	return pages, nil
}
