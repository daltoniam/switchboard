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

// normalizeHandle produces a comparison key for a phone number or email.
// Phone numbers compare on their last 10 digits so "+1 (555) 123-4567" and
// "5551234567" match.
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
	merged := map[string]*contact{}
	var opened int
	var lastErr error
	for _, path := range addressBookPaths(dir) {
		db, err := openDB(ctx, path)
		if err != nil {
			lastErr = err
			continue
		}
		opened++
		err = readAddressBook(ctx, db, merged)
		_ = db.Close()
		if err != nil {
			lastErr = err
		}
	}
	if opened == 0 {
		if lastErr == nil {
			lastErr = fmt.Errorf("no AddressBook databases found")
		}
		return nil, permissionHint(dir, lastErr)
	}
	out := make([]contact, 0, len(merged))
	for _, c := range merged {
		out = append(out, *c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func readAddressBook(ctx context.Context, db *sql.DB, merged map[string]*contact) error {
	rows, err := db.QueryContext(ctx, `
SELECT r.Z_PK, COALESCE(r.ZFIRSTNAME, ''), COALESCE(r.ZLASTNAME, ''),
       COALESCE(r.ZNICKNAME, ''), COALESCE(r.ZORGANIZATION, ''),
       'phone', COALESCE(p.ZFULLNUMBER, '')
FROM ZABCDRECORD r JOIN ZABCDPHONENUMBER p ON p.ZOWNER = r.Z_PK
UNION ALL
SELECT r.Z_PK, COALESCE(r.ZFIRSTNAME, ''), COALESCE(r.ZLASTNAME, ''),
       COALESCE(r.ZNICKNAME, ''), COALESCE(r.ZORGANIZATION, ''),
       'email', COALESCE(e.ZADDRESS, '')
FROM ZABCDRECORD r JOIN ZABCDEMAILADDRESS e ON e.ZOWNER = r.Z_PK`)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var pk int64
		var first, last, nick, org, kind, value string
		if err := rows.Scan(&pk, &first, &last, &nick, &org, &kind, &value); err != nil {
			return err
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
		key := strings.ToLower(name)
		c, ok := merged[key]
		if !ok {
			c = &contact{Name: name}
			merged[key] = c
		}
		switch kind {
		case "phone":
			c.Phones = appendUnique(c.Phones, value)
		case "email":
			c.Emails = appendUnique(c.Emails, value)
		}
	}
	return rows.Err()
}

func appendUnique(list []string, v string) []string {
	k := normalizeHandle(v)
	for _, existing := range list {
		if normalizeHandle(existing) == k {
			return list
		}
	}
	return append(list, v)
}
