package database

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/derat/aread/common"
	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	db *sql.DB
}

func New(path string) (*DB, error) {
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
			Id STRING NOT NULL,
			TimeAdded INTEGER,
			IpAddress STRING)`,
	} {
		if _, err = db.Exec(q); err != nil {
			return nil, fmt.Errorf("unable to initialize database: %v", err)
		}
	}

	return &DB{db: db}, nil
}

func (d *DB) ValidSession(id string) (bool, error) {
	rows, err := d.db.Query("SELECT * FROM Sessions WHERE Id = ?", id)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	return rows.Next(), nil
}

func (d *DB) AddSession(id, ip string) error {
	if _, err := d.db.Exec("INSERT OR REPLACE INTO Sessions (Id, TimeAdded, IpAddress) VALUES(?, ?, ?)", id, time.Now().Unix(), ip); err != nil {
		return err
	}
	return nil
}

func (d *DB) AddPage(pi common.PageInfo) error {
	q := "INSERT OR REPLACE INTO Pages (Id, OriginalUrl, Title, TimeAdded, Token) VALUES(?, ?, ?, ?, ?)"
	if _, err := d.db.Exec(q, pi.Id, pi.OriginalURL, pi.Title, pi.TimeAdded, pi.Token); err != nil {
		return err
	}
	return nil
}

func (d *DB) GetPage(id string) (pi common.PageInfo, err error) {
	rows, err := d.db.Query("SELECT Id, OriginalUrl, Title, TimeAdded, Token FROM Pages WHERE Id = ?", id)
	if err != nil {
		return pi, err
	}
	defer rows.Close()
	if !rows.Next() {
		return pi, errors.New("page not found in database")
	}
	if err = rows.Scan(&pi.Id, &pi.OriginalURL, &pi.Title, &pi.TimeAdded, &pi.Token); err != nil {
		return pi, err
	}
	return pi, nil
}

func (d *DB) GetAllPages(archived bool, maxPages int) (pages []common.PageInfo, err error) {
	q := "SELECT Id, OriginalUrl, Title, TimeAdded, Token FROM Pages WHERE Archived = ? ORDER BY TimeAdded DESC LIMIT ?"
	rows, err := d.db.Query(q, archived, maxPages)
	if err != nil {
		return pages, err
	}
	defer rows.Close()
	for rows.Next() {
		pi := common.PageInfo{}
		if err = rows.Scan(&pi.Id, &pi.OriginalURL, &pi.Title, &pi.TimeAdded, &pi.Token); err != nil {
			return pages, err
		}
		pages = append(pages, pi)
	}
	return pages, nil
}

func (d *DB) TogglePageArchived(id string) error {
	if _, err := d.db.Exec("UPDATE Pages SET Archived = (Archived != 1) WHERE Id = ?", id); err != nil {
		return err
	}
	return nil
}
