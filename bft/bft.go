package bft

import (
	"database/sql"
	"log"
	"strconv"

	_ "github.com/mattn/go-sqlite3"
)

const path string = "../bft.db"

var db *sql.DB

func init() {
	var err error
	db, err = sql.Open("sqlite3", path)
	if err != nil {
		log.Fatal(err)
	}
	// init db if doesn't exist
	statement, _ := db.Prepare("CREATE TABLE IF NOT EXISTS broadcast (id INTEGER PRIMARY KEY, data BLOB)")
	_, err = statement.Exec()
	if err != nil {
		log.Fatal(err)
	}
	statement, _ = db.Prepare("CREATE TABLE IF NOT EXISTS state (id INTEGER PRIMARY KEY, label TEXT, number INT, string TEXT)")
	_, err = statement.Exec()
	if err != nil {
		log.Fatal(err)
	}
	statement, _ = db.Prepare("INSERT INTO state (id, label, number, string) SELECT 1, 'epoch', 0, NULL WHERE NOT EXISTS (SELECT * FROM state)")
	_, err = statement.Exec()
	if err != nil {
		log.Fatal(err)
	}
}

func Epoch() int {
	row := db.QueryRow("SELECT number FROM state WHERE label='epoch'")
	var epoch int
	err := row.Scan(&epoch)
	if err != nil {
		log.Fatal(err)
	}
	return epoch
}

func SetEpoch(val int) sql.Result {
	row := db.QueryRow("SELECT id FROM state WHERE label='epoch'")
	var id int
	err := row.Scan(&id)
	if err != nil {
		log.Fatal(err)
	}
	statement, _ := db.Prepare("INSERT OR REPLACE INTO state (id, label, number, string) VALUES (" + strconv.Itoa(id) + ", 'epoch', " + strconv.Itoa(val) + ", NULL)")
	var res sql.Result
	res, err = statement.Exec()
	if err != nil {
		log.Fatal(err)
	}
	return res
}

func Broadcast(data []byte) sql.Result {
	statement, _ := db.Prepare("INSERT INTO broadcast (data) VALUES (?)")
	res, err := statement.Exec(data)
	if err != nil {
		log.Fatal(err)
	}
	return res
}
