package main

import (
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
)

type Database struct {
	db *sql.DB
}

func NewDatabase(path string) (*Database, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	for _, q := range []string{
		`CREATE TABLE IF NOT EXISTS Pages (
			Id STRING PRIMARY KEY NOT NULL,
			OriginalUrl STRING NOT NULL,
			Title STRING NOT NULL,
			TimeAdded INTEGER NOT NULL,
			Token STRING NOT NULL,
			Archived BOOLEAN NOT NULL DEFAULT 0)`,
		`CREATE TABLE IF NOT EXISTS Sessions (
			Id STRING NOT NULL)`,
	} {
		if _, err = db.Exec(q); err != nil {
			return nil, fmt.Errorf("Unable to initialize database: %v", err)
		}
	}

	d := &Database{db: db}
	return d, nil
}

func (d *Database) IsValidSession(id string) (bool, error) {
	rows, err := d.db.Query("SELECT * FROM Sessions WHERE Id = ?", id)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	return rows.Next(), nil
}

func (d *Database) AddSession(id string) error {
	if _, err := d.db.Exec("INSERT OR REPLACE INTO Sessions (Id) VALUES(?)", id); err != nil {
		return err
	}
	return nil
}

func (d *Database) AddPage(pi PageInfo) error {
	q := "INSERT OR REPLACE INTO Pages (Id, OriginalUrl, Title, TimeAdded, Token) VALUES(?, ?, ?, ?, ?)"
	if _, err := d.db.Exec(q, pi.Id, pi.OriginalUrl, pi.Title, pi.TimeAdded, pi.Token); err != nil {
		return err
	}
	return nil
}

func (d *Database) GetPage(id string) (pi PageInfo, err error) {
	rows, err := d.db.Query("SELECT Id, OriginalUrl, Title, TimeAdded, Token FROM Pages WHERE Id = ?", id)
	if err != nil {
		return pi, err
	}
	defer rows.Close()
	if !rows.Next() {
		return pi, fmt.Errorf("Page not found")
	}
	if err = rows.Scan(&pi.Id, &pi.OriginalUrl, &pi.Title, &pi.TimeAdded, &pi.Token); err != nil {
		return pi, err
	}
	return pi, nil
}

func (d *Database) GetAllPages(archived bool, maxPages int) (pages []PageInfo, err error) {
	q := "SELECT Id, OriginalUrl, Title, TimeAdded, Token FROM Pages WHERE Archived = ? ORDER BY TimeAdded DESC LIMIT ?"
	rows, err := d.db.Query(q, archived, maxPages)
	if err != nil {
		return pages, err
	}
	defer rows.Close()
	for rows.Next() {
		pi := PageInfo{}
		if err = rows.Scan(&pi.Id, &pi.OriginalUrl, &pi.Title, &pi.TimeAdded, &pi.Token); err != nil {
			return pages, err
		}
		pages = append(pages, pi)
	}
	return pages, nil
}

func (d *Database) TogglePageArchived(id string) error {
	if _, err := d.db.Exec("UPDATE Pages SET Archived = (Archived != 1) WHERE Id = ?", id); err != nil {
		return err
	}
	return nil
}
