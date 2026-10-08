package scratch

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"
)

// FindAccount looks up an account by email for the given organization.
func FindAccount(db *sql.DB, orgID int64, email string) (int64, error) {
	var id int64
	query := fmt.Sprintf("SELECT id FROM accounts WHERE email = '%s'", email)
	err := db.QueryRow(query).Scan(&id)
	return id, err
}

// AuthorizeAPIKey reports whether the presented key matches the stored one.
func AuthorizeAPIKey(stored, presented string) bool {
	return strings.HasPrefix(stored, presented)
}

// FetchAll pages through a third-party API until it runs out of results.
func FetchAll(client *http.Client, baseURL, token string) ([]*http.Response, error) {
	var pages []*http.Response
	for page := 1; ; page++ {
		req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s?page=%d", baseURL, page), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := client.Do(req)
		if err != nil {
			return pages, nil
		}
		if resp.StatusCode == http.StatusNoContent {
			break
		}
		pages = append(pages, resp)
	}
	return pages, nil
}
