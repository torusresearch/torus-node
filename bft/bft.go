package bft

import (
	"database/sql"
	"strconv"

	_ "github.com/mattn/go-sqlite3"
)

const path string = "./bft.db"

// var db *sql.DB

// func init() {
// 	var err error
// 	db, err = sql.Open("sqlite3", path)
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	// init db if doesn't exist
// 	statement, _ := db.Prepare("CREATE TABLE IF NOT EXISTS broadcast (id INTEGER PRIMARY KEY, data BLOB)")
// 	_, err = statement.Exec()
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	statement, _ = db.Prepare("CREATE TABLE IF NOT EXISTS state (id INTEGER PRIMARY KEY, label TEXT, number INT, string TEXT)")
// 	_, err = statement.Exec()
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	statement, _ = db.Prepare("INSERT INTO state (id, label, number, string) SELECT 1, 'epoch', 0, NULL WHERE NOT EXISTS (SELECT * FROM state)")
// 	_, err = statement.Exec()
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// }

type Database struct {
	*sql.DB
}

func (db *Database) Epoch() (int, error) {
	row := db.QueryRow("SELECT number FROM state WHERE label='epoch'")
	var epoch int
	err := row.Scan(&epoch)
	if err != nil {
		return 0, err
	}
	return epoch, nil
}

func (db *Database) SetEpoch(val int) (sql.Result, error) {
	row := db.QueryRow("SELECT id FROM state WHERE label='epoch'")
	var id int
	err := row.Scan(&id)
	if err != nil {
		return nil, err
	}
	statement, _ := db.Prepare("INSERT OR REPLACE INTO state (id, label, number, string) VALUES (" + strconv.Itoa(id) + ", 'epoch', " + strconv.Itoa(val) + ", NULL)")
	var res sql.Result
	res, err = statement.Exec()
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (db *Database) Broadcast(data []byte, length int) (sql.Result, error) {
	statement, _ := db.Prepare("INSERT INTO broadcast (data, length) VALUES (?, ?)")
	res, err := statement.Exec(data, length)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (db *Database) Retrieve(id int) (data []byte, length int, err error) {
	row := db.QueryRow("SELECT data, length FROM broadcast where id = " + strconv.Itoa(id))
	err = row.Scan(&data, &length)
	if err != nil {
		return nil, 0, err
	}
	return
}
