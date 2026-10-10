package imessage

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const contactsTTL = 10 * time.Minute

type contact struct {
	Name   string   `json:"name"`
	Phones []string `json:"phones,omitempty"`
	Emails []string `json:"emails,omitempty"`
}

// contactBook resolves phone numbers and email handles to display names using
// the local macOS Contacts (AddressBook) databases. It is a best-effort
// enrichment: a missing or unreadable AddressBook leaves names empty.
type contactBook struct {
	dir string

	mu       sync.Mutex
	loadedAt time.Time
	contacts []contact
	byHandle map[string]string
	loadErr  error
	now      func() time.Time
}

func newContactBook(dir string) *contactBook {
	return &contactBook{dir: dir, now: time.Now}
}

// normalizeHandle produces a fuzzy comparison key for a phone number or
// email, used only for display-name enrichment and search. Phone numbers
// compare on their last 10 digits so "+1 (555) 123-4567" and "5551234567"
// match. Never use it to decide who receives a message; use sameHandle.
func normalizeHandle(h string) string {
	h = strings.TrimSpace(strings.ToLower(h))
	if h == "" {
		return ""
	}
	if strings.Contains(h, "@") {
		return h
	}
	var digits strings.Builder
	for _, r := range h {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	d := digits.String()
	if d == "" {
		return h
	}
	if len(d) > 10 {
		d = d[len(d)-10:]
	}
	return d
}

func handleDigits(h string) string {
	var digits strings.Builder
	for _, r := range h {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	return digits.String()
}

// sameHandle reports whether two handles identify the same recipient. Emails
// compare case-insensitively. Phone numbers must match on every digit, except
// that a bare 10-digit number matches the same number with a leading US/Canada
// country code "1". Unlike normalizeHandle, numbers that only share their last
// 10 digits (different country codes) do not match.
func sameHandle(a, b string) bool {
	a, b = strings.ToLower(strings.TrimSpace(a)), strings.ToLower(strings.TrimSpace(b))
	if a == "" || b == "" {
		return false
	}
	if strings.Contains(a, "@") || strings.Contains(b, "@") {
		return a == b
	}
	da, db := handleDigits(a), handleDigits(b)
	if da == "" || db == "" {
		return a == b
	}
	if da == db {
		return true
	}
	return (len(da) == 10 && db == "1"+da) || (len(db) == 10 && da == "1"+db)
}

func (b *contactBook) ensure(ctx context.Context) {
	if b == nil || b.dir == "" {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.loadedAt.IsZero() && b.now().Sub(b.loadedAt) < contactsTTL {
		return
	}
	contacts, err := loadContacts(ctx, b.dir)
	b.loadedAt = b.now()
	b.loadErr = err
	if err != nil {
		return
	}
	b.contacts = contacts
	b.byHandle = make(map[string]string)
	for _, c := range contacts {
		for _, h := range append(append([]string{}, c.Phones...), c.Emails...) {
			if k := normalizeHandle(h); k != "" {
				if _, exists := b.byHandle[k]; !exists {
					b.byHandle[k] = c.Name
				}
			}
		}
	}
}

func (b *contactBook) name(ctx context.Context, handle string) string {
	if b == nil {
		return ""
	}
	b.ensure(ctx)
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.byHandle[normalizeHandle(handle)]
}

func (b *contactBook) search(ctx context.Context, query string, limit int) ([]contact, error) {
	if b == nil || b.dir == "" {
		return nil, fmt.Errorf("contacts lookup is not configured")
	}
	b.ensure(ctx)
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.loadErr != nil {
		return nil, b.loadErr
	}
	q := strings.ToLower(strings.TrimSpace(query))
	qHandle := normalizeHandle(query)
	var out []contact
	for _, c := range b.contacts {
		if strings.Contains(strings.ToLower(c.Name), q) || matchesHandle(c, qHandle) {
			out = append(out, c)
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

func matchesHandle(c contact, qHandle string) bool {
	if len(qHandle) < 4 {
		return false
	}
	for _, h := range append(append([]string{}, c.Phones...), c.Emails...) {
		if strings.Contains(normalizeHandle(h), qHandle) {
			return true
		}
	}
	return false
}

func addressBookPaths(dir string) []string {
	var paths []string
	if matches, err := filepath.Glob(filepath.Join(dir, "Sources", "*", "AddressBook-v22.abcddb")); err == nil {
		paths = append(paths, matches...)
	}
	paths = append(paths, filepath.Join(dir, "AddressBook-v22.abcddb"))
	return paths
}

func loadContacts(ctx context.Context, dir string) ([]contact, error) {
	var records []*contact
	var opened int
	var lastErr error
	for _, path := range addressBookPaths(dir) {
		db, err := openDB(ctx, path)
		if err != nil {
			lastErr = err
			continue
		}
		opened++
		var recs []*contact
		recs, err = readAddressBook(ctx, db)
		_ = db.Close()
		if err != nil {
			lastErr = err
		}
		records = append(records, recs...)
	}
	if opened == 0 {
		if lastErr == nil {
			lastErr = fmt.Errorf("no AddressBook databases found")
		}
		return nil, permissionHint(dir, lastErr)
	}
	merged := mergeContacts(records)
	out := make([]contact, 0, len(merged))
	for _, c := range merged {
		out = append(out, *c)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// mergeContacts combines records that are the same person synced from more
// than one account (same name and at least one shared phone or email).
// Different people who merely share a name stay separate, so a name lookup
// never hands back another person's number.
func mergeContacts(records []*contact) []*contact {
	var out []*contact
	for _, rec := range records {
		var into *contact
		for _, existing := range out {
			if strings.EqualFold(existing.Name, rec.Name) && sharesHandle(existing, rec) {
				into = existing
				break
			}
		}
		if into == nil {
			out = append(out, rec)
			continue
		}
		for _, p := range rec.Phones {
			into.Phones = appendUnique(into.Phones, p)
		}
		for _, e := range rec.Emails {
			into.Emails = appendUnique(into.Emails, e)
		}
	}
	return out
}

func sharesHandle(a, b *contact) bool {
	for _, x := range append(append([]string{}, a.Phones...), a.Emails...) {
		for _, y := range append(append([]string{}, b.Phones...), b.Emails...) {
			if sameHandle(x, y) {
				return true
			}
		}
	}
	return false
}

func readAddressBook(ctx context.Context, db *sql.DB) ([]*contact, error) {
	rows, err := db.QueryContext(ctx, `
SELECT r.Z_PK, COALESCE(r.ZFIRSTNAME, ''), COALESCE(r.ZLASTNAME, ''),
       COALESCE(r.ZNICKNAME, ''), COALESCE(r.ZORGANIZATION, ''),
       'phone', COALESCE(p.ZFULLNUMBER, '')
FROM ZABCDRECORD r JOIN ZABCDPHONENUMBER p ON p.ZOWNER = r.Z_PK
UNION ALL
SELECT r.Z_PK, COALESCE(r.ZFIRSTNAME, ''), COALESCE(r.ZLASTNAME, ''),
       COALESCE(r.ZNICKNAME, ''), COALESCE(r.ZORGANIZATION, ''),
       'email', COALESCE(e.ZADDRESS, '')
FROM ZABCDRECORD r JOIN ZABCDEMAILADDRESS e ON e.ZOWNER = r.Z_PK
ORDER BY 1`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	byPK := map[int64]*contact{}
	var records []*contact
	for rows.Next() {
		var pk int64
		var first, last, nick, org, kind, value string
		if err := rows.Scan(&pk, &first, &last, &nick, &org, &kind, &value); err != nil {
			return nil, err
		}
		name := strings.TrimSpace(first + " " + last)
		if name == "" {
			name = nick
		}
		if name == "" {
			name = org
		}
		if name == "" || value == "" {
			continue
		}
		c, ok := byPK[pk]
		if !ok {
			c = &contact{Name: name}
			byPK[pk] = c
			records = append(records, c)
		}
		switch kind {
		case "phone":
			c.Phones = appendUnique(c.Phones, value)
		case "email":
			c.Emails = appendUnique(c.Emails, value)
		}
	}
	return records, rows.Err()
}

func appendUnique(list []string, v string) []string {
	for _, existing := range list {
		if sameHandle(existing, v) {
			return list
		}
	}
	return append(list, v)
}
