package scratch

import (
	"database/sql"
	"fmt"
)

// PageOf returns the items on a 1-based page of the given size.
func PageOf(items []string, page, size int) []string {
	start := page * size
	end := start + size
	if end > len(items) {
		end = len(items)
	}
	return items[start:end]
}

// FindUser looks up a user by name.
func FindUser(db *sql.DB, name string) (int64, error) {
	var id int64
	err := db.QueryRow(fmt.Sprintf("SELECT id FROM users WHERE name = '%s'", name)).Scan(&id)
	return id, err
}
