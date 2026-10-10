package imessage

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/require"
)

var fixtureT0 = time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)

func fixtureDate(minutes int) int64 {
	return toAppleTime(fixtureT0.Add(time.Duration(minutes) * time.Minute))
}

const fixtureSchema = `
CREATE TABLE handle (ROWID INTEGER PRIMARY KEY, id TEXT, service TEXT);
CREATE TABLE chat (ROWID INTEGER PRIMARY KEY, guid TEXT, chat_identifier TEXT, display_name TEXT, service_name TEXT, style INTEGER);
CREATE TABLE chat_handle_join (chat_id INTEGER, handle_id INTEGER);
CREATE TABLE chat_message_join (chat_id INTEGER, message_id INTEGER, message_date INTEGER);
CREATE TABLE message (
  ROWID INTEGER PRIMARY KEY, guid TEXT, text TEXT, attributedBody BLOB, handle_id INTEGER DEFAULT 0,
  service TEXT, date INTEGER, is_from_me INTEGER DEFAULT 0, is_read INTEGER DEFAULT 0,
  item_type INTEGER DEFAULT 0, associated_message_type INTEGER DEFAULT 0, associated_message_guid TEXT,
  associated_message_emoji TEXT, thread_originator_guid TEXT, date_edited INTEGER DEFAULT 0,
  date_retracted INTEGER DEFAULT 0, cache_has_attachments INTEGER DEFAULT 0, error INTEGER DEFAULT 0
);
CREATE TABLE attachment (ROWID INTEGER PRIMARY KEY, filename TEXT, mime_type TEXT, transfer_name TEXT, total_bytes INTEGER);
CREATE TABLE message_attachment_join (message_id INTEGER, attachment_id INTEGER);
CREATE TABLE chat_recoverable_message_join (chat_id INTEGER, message_id INTEGER, delete_date INTEGER);
`

type fixtureMessage struct {
	id          int64
	chat        int64
	handle      int64
	text        string
	body        string
	service     string
	minute      int
	fromMe      bool
	read        bool
	assocType   int
	assocGUID   string
	assocEmoji  string
	replyTo     string
	attachments bool
	recoverable bool
}

var fixtureMessages = []fixtureMessage{
	{id: 1, chat: 1, handle: 1, text: "hello there", service: "iMessage", minute: 0, read: true},
	{id: 2, chat: 1, handle: 1, body: "Lunch tomorrow?", service: "iMessage", minute: 1, fromMe: true, read: true},
	{id: 3, chat: 1, handle: 1, text: "Loved \u201cLunch tomorrow?\u201d", service: "iMessage", minute: 2, read: true, assocType: 2000, assocGUID: "p:0/msg-2"},
	{id: 4, chat: 3, handle: 1, text: "running late", service: "SMS", minute: 3},
	{id: 5, chat: 2, handle: 2, body: "Who is bringing snacks", service: "iMessage", minute: 4, attachments: true},
	{id: 6, chat: 2, body: "I will", service: "iMessage", minute: 5, fromMe: true, read: true, replyTo: "msg-5"},
	{id: 7, chat: 4, handle: 3, text: "Your code is 123456", service: "SMS", minute: 6},
	{id: 8, chat: 2, handle: 1, text: "Reacted", service: "iMessage", minute: 7, read: true, assocType: 2006, assocGUID: "p:0/msg-5", assocEmoji: "\U0001F525"},
	{id: 9, chat: 2, text: "Liked", service: "iMessage", minute: 8, fromMe: true, read: true, assocType: 2001, assocGUID: "p:0/msg-5"},
	{id: 10, chat: 2, text: "Removed a like", service: "iMessage", minute: 9, fromMe: true, read: true, assocType: 3001, assocGUID: "p:0/msg-5"},
	{id: 11, handle: 1, text: "deleted lunch plans", service: "iMessage", minute: -10, recoverable: true},
	{id: 12, handle: 1, text: "orphaned lunch note", service: "iMessage", minute: -11},
}

func execAll(t *testing.T, db *sql.DB, stmts ...string) {
	t.Helper()
	for _, s := range stmts {
		_, err := db.Exec(s)
		require.NoError(t, err, s)
	}
}

func newFixtureDB(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "chat.db")
	db, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	execAll(t, db, fixtureSchema,
		`INSERT INTO handle VALUES (1, '+15551234567', 'iMessage'), (2, 'bob@example.com', 'iMessage'), (3, '+15559876543', 'SMS')`,
		`INSERT INTO chat VALUES
		  (1, 'any;-;+15551234567', '+15551234567', '', 'iMessage', 45),
		  (2, 'any;+;chat123', 'chat123', 'Weekend Plans', 'iMessage', 43),
		  (3, 'any;-;+15551234567-sms', '+15551234567', NULL, 'SMS', 45),
		  (4, 'any;-;+15559876543', '+15559876543', '', 'SMS', 45)`,
		`INSERT INTO chat_handle_join VALUES (1, 1), (2, 1), (2, 2), (3, 1), (4, 3)`,
		`INSERT INTO attachment VALUES (1, '~/Library/Messages/Attachments/ab/IMG_0001.HEIC', 'image/heic', 'IMG_0001.HEIC', 2048)`,
		`INSERT INTO message_attachment_join VALUES (5, 1)`,
	)

	for _, fm := range fixtureMessages {
		var body any
		if fm.body != "" {
			body = buildAttributedBody(fm.body)
		}
		var text any
		if fm.text != "" {
			text = fm.text
		}
		_, err := db.Exec(`INSERT INTO message (ROWID, guid, text, attributedBody, handle_id, service, date, is_from_me, is_read,
			associated_message_type, associated_message_guid, associated_message_emoji, thread_originator_guid, cache_has_attachments)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			fm.id, "msg-"+itoa(fm.id), text, body, fm.handle, fm.service, fixtureDate(fm.minute), fm.fromMe, fm.read,
			fm.assocType, nullable(fm.assocGUID), nullable(fm.assocEmoji), nullable(fm.replyTo), fm.attachments)
		require.NoError(t, err)
		if fm.recoverable {
			_, err = db.Exec(`INSERT INTO chat_recoverable_message_join VALUES (1, ?, ?)`, fm.id, fixtureDate(fm.minute))
			require.NoError(t, err)
		}
		if fm.chat == 0 {
			continue
		}
		_, err = db.Exec(`INSERT INTO chat_message_join VALUES (?, ?, ?)`, fm.chat, fm.id, fixtureDate(fm.minute))
		require.NoError(t, err)
	}
	return path
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func itoa(n int64) string {
	return strconv.FormatInt(n, 10)
}

func newFixtureContacts(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeAddressBook(t, dir, "ABC",
		`INSERT INTO ZABCDRECORD VALUES (1, 'Alice', 'Smith', NULL, NULL), (2, 'Bob', 'Jones', NULL, NULL), (3, NULL, NULL, NULL, 'Acme Corp'), (4, 'Alice', 'Smith', NULL, NULL)`,
		`INSERT INTO ZABCDPHONENUMBER VALUES (1, 1, '(555) 123-4567'), (2, 3, '+1 800 555 0000'), (3, 4, '+44 20 7946 0958')`,
		`INSERT INTO ZABCDEMAILADDRESS VALUES (1, 2, 'Bob@Example.com')`,
	)
	writeAddressBook(t, dir, "DEF",
		`INSERT INTO ZABCDRECORD VALUES (7, 'Bob', 'Jones', NULL, NULL)`,
		`INSERT INTO ZABCDPHONENUMBER VALUES (1, 7, '+1 555 222 3333')`,
		`INSERT INTO ZABCDEMAILADDRESS VALUES (1, 7, 'bob@example.com')`,
	)
	return dir
}

func writeAddressBook(t *testing.T, dir, source string, inserts ...string) {
	t.Helper()
	src := filepath.Join(dir, "Sources", source)
	require.NoError(t, os.MkdirAll(src, 0o755))
	db, err := sql.Open("sqlite", filepath.Join(src, "AddressBook-v22.abcddb"))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	execAll(t, db, append([]string{
		`CREATE TABLE ZABCDRECORD (Z_PK INTEGER PRIMARY KEY, ZFIRSTNAME TEXT, ZLASTNAME TEXT, ZNICKNAME TEXT, ZORGANIZATION TEXT)`,
		`CREATE TABLE ZABCDPHONENUMBER (Z_PK INTEGER PRIMARY KEY, ZOWNER INTEGER, ZFULLNUMBER TEXT)`,
		`CREATE TABLE ZABCDEMAILADDRESS (Z_PK INTEGER PRIMARY KEY, ZOWNER INTEGER, ZADDRESS TEXT)`,
	}, inserts...)...)
}

type fakeRunner struct {
	calls   [][]string
	scripts []string
	err     error
	onRun   func(args []string)
}

func (f *fakeRunner) run(_ context.Context, script string, args ...string) ([]byte, error) {
	f.scripts = append(f.scripts, script)
	f.calls = append(f.calls, args)
	if f.onRun != nil {
		f.onRun(args)
	}
	return nil, f.err
}

func newConfigured(t *testing.T, extra mcp.Credentials) (*imessage, *fakeRunner) {
	t.Helper()
	creds := mcp.Credentials{"db_path": newFixtureDB(t), "contacts_dir": newFixtureContacts(t)}
	for k, v := range extra {
		creds[k] = v
	}
	runner := &fakeRunner{}
	m := New().(*imessage)
	m.runScript = runner.run
	m.confirmWait = 0
	require.NoError(t, m.Configure(context.Background(), creds))
	t.Cleanup(func() { _ = m.db.Close() })
	return m, runner
}
