package db

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

const createTestTable = `
CREATE table test (id INTEGER NOT NULL primary key, key INTEGER, value INTEGER )
`

// Store represents persistent customer and event storage.
type SqliteDB struct {
	path string
	db   *sql.DB
}

// New returns a new sql database.
func NewSqliteDB(path string) (*SqliteDB, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	s := &SqliteDB{
		path: path,
		db:   db,
	}

	err = s.init()
	if err != nil {
		return s, err
	}

	return s, nil
}

func (s *SqliteDB) init() error {
	_, err := s.db.Exec(createTestTable)
	if err != nil {
		return err
	}
	return nil
}

func (s *SqliteDB) Get(key int64) (int64, error) {
	row := s.db.QueryRow("SELECT value FROM test WHERE key=$1;", key)
	var res int64

	err := row.Scan(&res)
	switch err {
	case sql.ErrNoRows:
		return 0, sql.ErrNoRows
	case nil:
		return res, nil
	default:
		panic(err)
	}
}

func (s *SqliteDB) Set(key, val int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare("INSERT into test(key, value) values(?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()
	_, err = stmt.Exec(key, val)
	if err != nil {
		return err
	}
	err = tx.Commit()
	if err != nil {
		return err
	}
	return nil
}
