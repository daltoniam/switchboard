package server

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// FileSessionStore persists sessions as markdown files in a directory.
// Each session is stored as {dir}/{id}.md with a YAML-ish front-matter
// section for context and a markdown body for the breadcrumb trail.
// This is the default durable store for the open-source version.
type FileSessionStore struct {
	dir string
	ttl time.Duration
	mu  sync.RWMutex
	mem map[string]*Session
}

// NewFileSessionStore creates a file-backed session store that writes
// markdown files to dir. The directory is created lazily on first write.
func NewFileSessionStore(dir string, ttl time.Duration) *FileSessionStore {
	if ttl <= 0 {
		ttl = DefaultSessionTTL
	}
	return &FileSessionStore{
		dir: dir,
		ttl: ttl,
		mem: make(map[string]*Session),
	}
}

func (fs *FileSessionStore) GetOrCreate(id string) *Session {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	if s, ok := fs.mem[id]; ok {
		if time.Since(s.LastUsed) <= fs.ttl {
			s.touch()
			return s
		}
		delete(fs.mem, id)
	}
	s := fs.loadFromDisk(id)
	if s != nil && time.Since(s.LastUsed) <= fs.ttl {
		fs.mem[id] = s
		s.touch()
		return s
	}
	s = newSession(id)
	fs.mem[id] = s
	return s
}

func (fs *FileSessionStore) Get(id string) (*Session, bool) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	if s, ok := fs.mem[id]; ok {
		if time.Since(s.LastUsed) <= fs.ttl {
			return s, true
		}
		return nil, false
	}
	s := fs.loadFromDisk(id)
	if s != nil && time.Since(s.LastUsed) <= fs.ttl {
		return s, true
	}
	return nil, false
}

func (fs *FileSessionStore) Save(s *Session) error {
	fs.mu.Lock()
	fs.mem[s.ID] = s
	fs.mu.Unlock()
	return fs.writeToDisk(s)
}

func (fs *FileSessionStore) Delete(id string) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	delete(fs.mem, id)
	_ = os.Remove(fs.filePath(id))
}

func (fs *FileSessionStore) filePath(id string) string {
	safe := sanitizeID(id)
	return filepath.Join(fs.dir, safe+".md")
}

func (fs *FileSessionStore) writeToDisk(s *Session) error {
	if err := os.MkdirAll(fs.dir, 0700); err != nil {
		return fmt.Errorf("create session dir: %w", err)
	}
	md := formatSessionMarkdown(s)
	tmp := fs.filePath(s.ID) + ".tmp"
	if err := os.WriteFile(tmp, []byte(md), 0600); err != nil {
		return fmt.Errorf("write session: %w", err)
	}
	return os.Rename(tmp, fs.filePath(s.ID))
}

func (fs *FileSessionStore) loadFromDisk(id string) *Session {
	data, err := os.ReadFile(fs.filePath(id))
	if err != nil {
		return nil
	}
	return parseSessionMarkdown(id, string(data))
}

func sanitizeID(id string) string {
	r := strings.NewReplacer("/", "_", "\\", "_", "..", "_", "\x00", "_")
	return r.Replace(id)
}

func formatSessionMarkdown(s *Session) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var b strings.Builder
	b.WriteString("# Session: ")
	b.WriteString(s.ID)
	b.WriteString("\n\n")

	b.WriteString("## Metadata\n\n")
	b.WriteString("- **Created:** ")
	b.WriteString(s.CreatedAt.Format(time.RFC3339))
	b.WriteString("\n")
	b.WriteString("- **Last Used:** ")
	b.WriteString(s.LastUsed.Format(time.RFC3339))
	b.WriteString("\n\n")

	b.WriteString("## Context\n\n")
	if len(s.Context) == 0 {
		b.WriteString("_No context set._\n\n")
	} else {
		for k, v := range s.Context {
			b.WriteString("- **")
			b.WriteString(k)
			b.WriteString(":** `")
			fmt.Fprint(&b, v)
			b.WriteString("`\n")
		}
		b.WriteString("\n")
	}

	b.WriteString("## Breadcrumbs\n\n")
	if len(s.Breadcrumbs) == 0 {
		b.WriteString("_No tool calls recorded._\n")
	} else {
		b.WriteString("| # | Time | Tool | Error | Summary |\n")
		b.WriteString("|---|------|------|-------|---------|\n")
		for _, bc := range s.Breadcrumbs {
			errMark := ""
			if bc.IsError {
				errMark = "yes"
			}
			summary := strings.ReplaceAll(bc.Summary, "|", "\\|")
			summary = strings.ReplaceAll(summary, "\n", " ")
			if len(summary) > 80 {
				summary = summary[:80] + "..."
			}
			fmt.Fprintf(&b, "| %d | %s | `%s` | %s | %s |\n",
				bc.Seq,
				bc.Timestamp.Format("15:04:05"),
				bc.Tool,
				errMark,
				summary,
			)
		}
	}

	b.WriteString("\n<!-- session-data\n")
	data := sessionData{
		Context:     s.Context,
		CreatedAt:   s.CreatedAt,
		LastUsed:    s.LastUsed,
		Breadcrumbs: s.Breadcrumbs,
		NextSeq:     s.nextSeq,
	}
	enc, _ := json.Marshal(data)
	b.Write(enc)
	b.WriteString("\nsession-data -->\n")

	return b.String()
}

type sessionData struct {
	Context     map[string]any `json:"context"`
	CreatedAt   time.Time      `json:"created_at"`
	LastUsed    time.Time      `json:"last_used"`
	Breadcrumbs []Breadcrumb   `json:"breadcrumbs,omitempty"`
	NextSeq     int            `json:"next_seq"`
}

func parseSessionMarkdown(id, content string) *Session {
	const startMarker = "<!-- session-data\n"
	const endMarker = "\nsession-data -->"

	startIdx := strings.Index(content, startMarker)
	if startIdx < 0 {
		return nil
	}
	startIdx += len(startMarker)
	endIdx := strings.Index(content[startIdx:], endMarker)
	if endIdx < 0 {
		return nil
	}
	jsonStr := content[startIdx : startIdx+endIdx]

	var data sessionData
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return nil
	}

	s := &Session{
		ID:          id,
		Context:     data.Context,
		CreatedAt:   data.CreatedAt,
		LastUsed:    data.LastUsed,
		Breadcrumbs: data.Breadcrumbs,
		nextSeq:     data.NextSeq,
	}
	if s.Context == nil {
		s.Context = make(map[string]any)
	}
	return s
}

// DefaultSessionDir returns the default directory for session files:
// ~/.config/switchboard/sessions
func DefaultSessionDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".switchboard", "sessions")
	}
	return filepath.Join(home, ".config", "switchboard", "sessions")
}
