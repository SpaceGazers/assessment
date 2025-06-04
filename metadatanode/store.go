package metadatanode

import (
	"database/sql"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	conn *sql.DB
}

func OpenDB(filename string) (*DB, error) {
	conn, err := sql.Open("sqlite3", filename)
	if err != nil {
		return nil, err
	}
	if _ , err := conn.Exec("CREATE TABLE file_blocks(blob, block)"); err != nil {
		log.Fatalln(err)
	}
	return &DB{conn}, err
}

// This is not concurrency-safe since SQLite3 is not
func (self *DB) Append(key, value string) error {
	_, err := self.conn.Exec("INSERT INTO file_blocks VALUES(?, ?)", key, value)
	return err
}

func (self *DB) Get(key string) ([]string, error) {
	rows, err := self.conn.Query("SELECT block FROM file_blocks WHERE blob=?", key)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var blocks []string
	for rows.Next() {
		var b string
		err = rows.Scan(&b)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, b)
	}

	return blocks, nil
}
