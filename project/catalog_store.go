package project

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	projectsDirName  = "projects"
	revisionsDirName = "revisions"
	pageSizeDefault  = 50
)

type catalogRecord struct {
	id                ProjectID
	path              string
	userBytes         []byte
	user              *Definition
	effective         *Definition
	revision          Revision
	sourceRevision    Revision
	rawSourceRevision Revision
	sources           []Source
	diagnostics       []Diagnostic
	invalid           bool
}

func (s *Store) projectsDir() string {
	return filepath.Join(s.configDir, projectsDirName)
}

func (s *Store) revisionsDir() string {
	return filepath.Join(s.configDir, revisionsDirName)
}

func (s *Store) ensureLayout() error {
	for _, dir := range []string{s.configDir, s.projectsDir(), s.revisionsDir()} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) rebuildIndex() error {
	if err := s.ensureLayout(); err != nil {
		return err
	}
	entries, err := os.ReadDir(s.projectsDir())
	if err != nil {
		return fmt.Errorf("reading projects dir: %w", err)
	}

	next := make(map[ProjectID]*catalogRecord)
	folded := make(map[string][]ProjectID)
	s.projects = make(map[string]*Definition)

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		id, ok := stemFromFileName(e.Name())
		if !ok {
			continue
		}
		path := filepath.Join(s.projectsDir(), e.Name())
		rec := s.readUserRecord(id, path)
		fold := strings.ToLower(string(id))
		folded[fold] = append(folded[fold], id)
		next[id] = rec
		if rec != nil && !rec.invalid && rec.effective != nil {
			s.projects[string(id)] = cloneDefinition(rec.effective)
		}
	}
	for _, ids := range folded {
		if len(ids) < 2 {
			continue
		}
		for _, id := range ids {
			rec := next[id]
			if rec == nil {
				continue
			}
			rec.invalid = true
			rec.effective = nil
			rec.revision = ""
			rec.diagnostics = append(rec.diagnostics, Diagnostic{
				Severity:  "error",
				Code:      CodeDuplicateProjectID,
				Message:   "duplicate project id on a case-insensitive filesystem",
				SourceURI: fileURI(rec.path),
			})
			delete(s.projects, string(id))
		}
	}
	s.index = next
	return nil
}

func (s *Store) readUserRecord(id ProjectID, path string) *catalogRecord {
	raw, err := os.ReadFile(path)
	rec := &catalogRecord{
		id:        id,
		path:      path,
		userBytes: append([]byte(nil), raw...),
	}
	if err != nil {
		rec.invalid = true
		rec.diagnostics = []Diagnostic{{
			Severity:  "error",
			Code:      CodeInvalidDefinition,
			Message:   "unable to read project definition",
			SourceURI: fileURI(path),
		}}
		return rec
	}
	rec.rawSourceRevision = RawSourceRevision(raw)
	var def Definition
	if err := json.Unmarshal(raw, &def); err != nil {
		rec.invalid = true
		rec.diagnostics = []Diagnostic{{
			Severity:  "error",
			Code:      CodeInvalidDefinition,
			Message:   "malformed project JSON",
			SourceURI: fileURI(path),
		}}
		return rec
	}
	user := cloneDefinition(&def)
	rec.user = user
	if user.Name == "" {
		rec.invalid = true
		rec.diagnostics = []Diagnostic{{
			Severity:  "error",
			Code:      CodeInvalidDefinition,
			Message:   "name is required",
			SourceURI: fileURI(path),
			Path:      "/name",
		}}
		return rec
	}
	if user.Name != string(id) {
		rec.invalid = true
		rec.diagnostics = []Diagnostic{{
			Severity:  "error",
			Code:      CodeInvalidDefinition,
			Message:   "embedded name must equal the filename stem",
			SourceURI: fileURI(path),
			Path:      "/name",
		}}
		return rec
	}
	if err := user.Validate(); err != nil {
		rec.invalid = true
		rec.diagnostics = []Diagnostic{{
			Severity:  "error",
			Code:      CodeInvalidDefinition,
			Message:   valueFreeValidationMessage(err),
			SourceURI: fileURI(path),
		}}
		return rec
	}
	srcRev, err := hashUserLayer(user)
	if err != nil {
		rec.invalid = true
		rec.diagnostics = []Diagnostic{{
			Severity:  "error",
			Code:      CodeInvalidDefinition,
			Message:   "definition cannot be canonicalized",
			SourceURI: fileURI(path),
		}}
		return rec
	}
	rec.sourceRevision = srcRev
	effective, sources, diags := s.mergeEffective(user, "", path)
	rec.sources = sources
	rec.diagnostics = append(rec.diagnostics, diags...)
	if hasError(diags) {
		rec.invalid = true
		return rec
	}
	rec.effective = effective
	rev, _, err := HashUserAndEffective(user, effective)
	if err != nil {
		rec.invalid = true
		rec.diagnostics = append(rec.diagnostics, Diagnostic{
			Severity:  "error",
			Code:      CodeInvalidDefinition,
			Message:   "effective definition cannot be canonicalized",
			SourceURI: fileURI(path),
		})
		return rec
	}
	rec.revision = rev
	return rec
}

func hashUserLayer(user *Definition) (Revision, error) {
	canon, err := CanonicalJSON(user)
	if err != nil {
		return "", err
	}
	return HashBytes(canon), nil
}

func valueFreeValidationMessage(err error) string {
	msg := err.Error()
	if strings.Contains(msg, "unsupported version") {
		return "unsupported version (must be 1)"
	}
	if strings.Contains(msg, "name is required") {
		return "name is required"
	}
	if strings.Contains(msg, "exceeds 128 characters") {
		return "name exceeds 128 characters"
	}
	if strings.Contains(msg, "does not match pattern") {
		return "name does not match the required pattern"
	}
	if strings.Contains(msg, "resources is required") {
		return "resources is required"
	}
	if strings.Contains(msg, "cannot set both repo and root") {
		return "files resource cannot set both repo and root"
	}
	if strings.Contains(msg, "does not reference a resource") || strings.Contains(msg, "must reference a resource with type repo") {
		return "repo reference is invalid"
	}
	return "invalid project definition"
}

func hasError(diags []Diagnostic) bool {
	for _, d := range diags {
		if d.Severity == "error" {
			return true
		}
	}
	return false
}

func (s *Store) mergeEffective(user *Definition, root, userPath string) (*Definition, []Source, []Diagnostic) {
	effective := cloneDefinition(user)
	sources := []Source{{
		Kind:       "user",
		URI:        fileURI(userPath),
		Precedence: 0,
	}}
	var diags []Diagnostic

	// Overlay applies when resolving a concrete root, or against the sole
	// primary repo resource. Multi-repo projects skip automatic overlay unless
	// an explicit root is provided (resolve path).
	overlayRoot := root
	if overlayRoot == "" {
		overlayRoot = user.ResolvedRepo()
	}
	if overlayRoot == "" {
		return effective, sources, diags
	}
	overlayPath := filepath.Join(overlayRoot, ".project.json")
	raw, err := os.ReadFile(overlayPath)
	if err != nil {
		return effective, sources, diags
	}
	var overlay Definition
	if err := json.Unmarshal(raw, &overlay); err != nil {
		diags = append(diags, Diagnostic{
			Severity:  "error",
			Code:      CodeInvalidDefinition,
			Message:   "malformed repository overlay JSON",
			SourceURI: fileURI(overlayPath),
		})
		return effective, sources, diags
	}
	if overlay.Name != "" && overlay.Name != user.Name {
		diags = append(diags, Diagnostic{
			Severity:  "error",
			Code:      CodeInvalidDefinition,
			Message:   "repository overlay name must equal the selected project id",
			SourceURI: fileURI(overlayPath),
			Path:      "/name",
		})
		return effective, sources, diags
	}
	merged, err := Merge(user, &overlay)
	if err != nil {
		diags = append(diags, Diagnostic{
			Severity:  "error",
			Code:      CodeInvalidDefinition,
			Message:   "repository overlay cannot be merged",
			SourceURI: fileURI(overlayPath),
		})
		return effective, sources, diags
	}
	effective = merged
	sources = append(sources, Source{
		Kind:       "repository",
		URI:        fileURI(overlayPath),
		Precedence: 1,
	})
	return effective, sources, diags
}

func (s *Store) snapshotFromRecord(rec *catalogRecord, root string) (Snapshot, error) {
	if rec == nil {
		return Snapshot{}, &Error{Code: CodeProjectNotFound, Message: "project not found"}
	}
	if rec.invalid || rec.effective == nil {
		return Snapshot{}, &Error{
			Code:        CodeInvalidDefinition,
			Message:     "project definition is invalid",
			ProjectID:   rec.id,
			Diagnostics: cloneDiagnostics(rec.diagnostics),
		}
	}
	snap := Snapshot{
		ProjectID:      rec.id,
		Revision:       rec.revision,
		SourceRevision: rec.sourceRevision,
		Definition:     *cloneDefinition(rec.effective),
		Sources:        append([]Source(nil), rec.sources...),
		Diagnostics:    cloneDiagnostics(rec.diagnostics),
		RootURI:        root,
		UserBytes:      append([]byte(nil), rec.userBytes...),
	}
	return snap, nil
}

func (s *Store) summaryFromRecord(rec *catalogRecord) (ProjectSummary, InvalidProjectSummary, bool) {
	if rec.invalid || rec.effective == nil {
		return ProjectSummary{}, InvalidProjectSummary{
			ProjectID:         rec.id,
			Title:             string(rec.id),
			RawSourceRevision: rec.rawSourceRevision,
			DiagnosticCount:   len(rec.diagnostics),
			DiagnosticsURI:    diagnosticsURI(rec.id),
			Diagnostics:       cloneDiagnostics(rec.diagnostics),
		}, false
	}
	return ProjectSummary{
		ProjectID:       rec.id,
		Title:           rec.effective.Name,
		Description:     rec.effective.Description,
		Revision:        rec.revision,
		SourceRevision:  rec.sourceRevision,
		DefinitionURI:   definitionURI(rec.id),
		DiagnosticCount: len(rec.diagnostics),
	}, InvalidProjectSummary{}, true
}

func (s *Store) sortedIDs() []ProjectID {
	ids := make([]ProjectID, 0, len(s.index))
	for id := range s.index {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func decodeCursor(cursor, kind, query, generation string) (ProjectID, error) {
	if cursor == "" {
		return "", nil
	}
	parts := strings.Split(cursor, "|")
	if len(parts) != 4 || parts[0] != kind || parts[1] != query || parts[2] != generation {
		return "", &Error{Code: CodeInvalidDefinition, Message: "cursor is stale or malformed"}
	}
	return ProjectID(parts[3]), nil
}

func encodeCursor(kind, query, generation string, last ProjectID) string {
	return strings.Join([]string{kind, query, generation, string(last)}, "|")
}

func (s *Store) generationDigest() string {
	ids := s.sortedIDs()
	var b strings.Builder
	for _, id := range ids {
		rec := s.index[id]
		b.WriteString(string(id))
		b.WriteByte(':')
		if rec.invalid {
			b.WriteString(string(rec.rawSourceRevision))
		} else {
			b.WriteString(string(rec.revision))
			b.WriteByte('/')
			b.WriteString(string(rec.sourceRevision))
		}
		b.WriteByte(';')
	}
	return string(HashBytes([]byte(b.String())))
}

func (s *Store) pageFrom(kind, query, cursor string) (Page, error) {
	if err := s.rebuildIndex(); err != nil {
		return Page{}, err
	}
	gen := s.generationDigest()
	after, err := decodeCursor(cursor, kind, query, gen)
	if err != nil {
		return Page{}, err
	}
	q := strings.ToLower(strings.TrimSpace(query))
	var valid []ProjectSummary
	var invalid []InvalidProjectSummary
	for _, id := range s.sortedIDs() {
		if after != "" && id <= after {
			continue
		}
		rec := s.index[id]
		if q != "" && !strings.Contains(strings.ToLower(string(id)), q) && (rec.effective == nil || !strings.Contains(strings.ToLower(rec.effective.TitleOrName()), q)) {
			if rec.effective == nil || !strings.Contains(strings.ToLower(rec.effective.Description), q) {
				continue
			}
		}
		sum, inv, ok := s.summaryFromRecord(rec)
		if ok {
			valid = append(valid, sum)
		} else {
			invalid = append(invalid, inv)
		}
	}
	page := Page{Projects: []ProjectSummary{}, InvalidProjects: []InvalidProjectSummary{}}
	remaining := append([]ProjectSummary(nil), valid...)
	if len(remaining) > pageSizeDefault {
		page.Projects = remaining[:pageSizeDefault]
		page.NextCursor = encodeCursor(kind, query, gen, remaining[pageSizeDefault-1].ProjectID)
	} else {
		page.Projects = remaining
	}
	// Invalids only on the first page so multi-page consumers (and listAllCatalog)
	// do not duplicate them on every cursor hop.
	if after == "" {
		page.InvalidProjects = invalid
	}
	return page, nil
}

func (d *Definition) TitleOrName() string {
	if d == nil {
		return ""
	}
	return d.Name
}

func (s *Store) List(ctx context.Context, cursor string) (Page, error) {
	if err := ctx.Err(); err != nil {
		return Page{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pageFrom("list", "", cursor)
}

func (s *Store) Search(ctx context.Context, req SearchRequest) (Page, error) {
	if err := ctx.Err(); err != nil {
		return Page{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pageFrom("search", req.Query, req.Cursor)
}

func (s *Store) Get(ctx context.Context, id ProjectID) (Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return Snapshot{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.rebuildIndex(); err != nil {
		return Snapshot{}, err
	}
	rec := s.index[id]
	if rec == nil {
		return Snapshot{}, errorWithProject(CodeProjectNotFound, "project not found", id)
	}
	return s.snapshotFromRecord(rec, "")
}

func (s *Store) GetRevision(ctx context.Context, id ProjectID, rev Revision) (RevisionSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return RevisionSnapshot{}, err
	}
	if !nameRE.MatchString(string(id)) {
		return RevisionSnapshot{}, &Error{Code: CodeInvalidDefinition, Message: "invalid project id", ProjectID: id}
	}
	parsed, err := ParseRevision(rev)
	if err != nil {
		return RevisionSnapshot{}, &Error{Code: CodeInvalidDefinition, Message: "invalid revision", ProjectID: id}
	}
	base := s.revisionsDir()
	path := filepath.Join(base, string(id), parsed.DigestHex()+".json")
	// Reject path escape even if Join cleans ".." segments.
	rel, err := filepath.Rel(base, filepath.Clean(path))
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return RevisionSnapshot{}, errorWithProject(CodeProjectNotFound, "revision not found", id)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return RevisionSnapshot{}, errorWithProject(CodeProjectNotFound, "revision not found", id)
	}
	var env revisionEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return RevisionSnapshot{}, &Error{Code: CodeInternalError, Message: "revision archive is unreadable", ProjectID: id}
	}
	return cloneRevisionSnapshot(RevisionSnapshot{
		ProjectID:  id,
		Revision:   parsed,
		Definition: env.Definition,
	}), nil
}

func (s *Store) Diagnostics(ctx context.Context, id ProjectID, root *url.URL) (DiagnosticsEnvelope, error) {
	if err := ctx.Err(); err != nil {
		return DiagnosticsEnvelope{}, err
	}
	s.mu.Lock()
	if err := s.rebuildIndex(); err != nil {
		s.mu.Unlock()
		return DiagnosticsEnvelope{}, err
	}
	rec := s.index[id]
	if rec == nil {
		return DiagnosticsEnvelope{}, errorWithProject(CodeProjectNotFound, "project not found", id)
	}
	if root == nil {
		env := DiagnosticsEnvelope{ProjectID: id, Diagnostics: cloneDiagnostics(rec.diagnostics)}
		s.mu.Unlock()
		return env, nil
	}
	s.mu.Unlock()
	snap, err := s.Resolve(ctx, ResolveRequest{ProjectID: id, RootURI: root.String()})
	if err != nil {
		if e, ok := AsError(err); ok && e.Code == CodeInvalidDefinition {
			return DiagnosticsEnvelope{ProjectID: id, Diagnostics: cloneDiagnostics(e.Diagnostics)}, nil
		}
		return DiagnosticsEnvelope{}, err
	}
	return DiagnosticsEnvelope{ProjectID: id, Diagnostics: cloneDiagnostics(snap.Diagnostics)}, nil
}

func (s *Store) Resolve(ctx context.Context, req ResolveRequest) (Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return Snapshot{}, err
	}
	if req.ProjectID == "" && req.RootURI == "" {
		return Snapshot{}, &Error{Code: CodeInvalidRoot, Message: "projectId or rootUri is required"}
	}

	var rootAbs string
	if req.RootURI != "" {
		abs, err := parseFileURI(req.RootURI)
		if err != nil {
			return Snapshot{}, err
		}
		rootAbs = abs
	}

	// Hold the store mutex while rebuilding/reading the index so Resolve cannot
	// race List/Get/Search (which also rebuild under s.mu).
	s.mu.Lock()
	if err := s.rebuildIndex(); err != nil {
		s.mu.Unlock()
		return Snapshot{}, err
	}

	id := req.ProjectID
	if id == "" {
		matched, matchErr := s.matchProjectByRoot(ctx, rootAbs)
		if matchErr != nil {
			s.mu.Unlock()
			return Snapshot{}, matchErr
		}
		id = matched
	} else if rootAbs != "" {
		matched, matchErr := s.matchProjectByRoot(ctx, rootAbs)
		if matchErr != nil && matchErr.Code != CodeProjectNotFound && matchErr.Code != CodeInvalidRoot {
			s.mu.Unlock()
			return Snapshot{}, matchErr
		}
		if matchErr == nil && matched != id {
			s.mu.Unlock()
			return Snapshot{}, &Error{Code: CodeRootProjectMismatch, Message: "rootUri does not belong to project", ProjectID: id, RootURI: req.RootURI}
		}
	}

	rec := s.index[id]
	if rec == nil {
		s.mu.Unlock()
		return Snapshot{}, errorWithProject(CodeProjectNotFound, "project not found", id)
	}
	if rec.invalid || rec.user == nil {
		diags := cloneDiagnostics(rec.diagnostics)
		s.mu.Unlock()
		return Snapshot{}, &Error{
			Code:        CodeInvalidDefinition,
			Message:     "project definition is invalid",
			ProjectID:   id,
			Diagnostics: diags,
		}
	}
	user := cloneDefinition(rec.user)
	userBytes := append([]byte(nil), rec.userBytes...)
	userPath := rec.path
	s.mu.Unlock()

	effective, sources, diags := s.mergeEffective(user, rootAbs, userPath)
	if hasError(diags) {
		return Snapshot{}, &Error{
			Code:        CodeInvalidDefinition,
			Message:     "project definition is invalid",
			ProjectID:   id,
			Diagnostics: cloneDiagnostics(diags),
			RootURI:     req.RootURI,
		}
	}
	rev, srcRev, err := HashUserAndEffective(user, effective)
	if err != nil {
		return Snapshot{}, &Error{Code: CodeInternalError, Message: err.Error(), ProjectID: id}
	}
	if err := s.archiveRevision(id, rev, effective); err != nil {
		return Snapshot{}, err
	}
	rootURI := ""
	if req.RootURI != "" {
		rootURI = fileURI(rootAbs)
	}
	return Snapshot{
		ProjectID:      id,
		Revision:       rev,
		SourceRevision: srcRev,
		Definition:     *cloneDefinition(effective),
		Sources:        sources,
		Diagnostics:    cloneDiagnostics(diags),
		RootURI:        rootURI,
		UserBytes:      userBytes,
	}, nil
}

func (s *Store) ValidateJSON(_ context.Context, raw json.RawMessage, root *url.URL) []Diagnostic {
	var def Definition
	if err := json.Unmarshal(raw, &def); err != nil {
		return []Diagnostic{{
			Severity: "error",
			Code:     CodeInvalidDefinition,
			Message:  "definition must be a JSON object",
		}}
	}
	if err := def.Validate(); err != nil {
		return []Diagnostic{{
			Severity: "error",
			Code:     CodeInvalidDefinition,
			Message:  valueFreeValidationMessage(err),
		}}
	}
	if root == nil {
		return []Diagnostic{}
	}
	abs, err := parseFileURI(root.String())
	if err != nil {
		if e, ok := AsError(err); ok {
			return []Diagnostic{{Severity: "error", Code: e.Code, Message: e.Message}}
		}
		return []Diagnostic{{Severity: "error", Code: CodeInvalidRoot, Message: "invalid rootUri"}}
	}
	_ = abs
	return []Diagnostic{}
}

func (s *Store) Create(ctx context.Context, req CreateRequest) (Snapshot, error) {
	var snap Snapshot
	err := s.withLock(ctx, func() error {
		if err := s.rebuildIndex(); err != nil {
			return err
		}
		def := cloneDefinition(&req.Definition)
		if err := def.Validate(); err != nil {
			return &Error{Code: CodeInvalidDefinition, Message: valueFreeValidationMessage(err)}
		}
		id := ProjectID(def.Name)
		if rec := s.index[id]; rec != nil {
			return errorWithProject(CodeProjectAlreadyExists, "project already exists", id)
		}
		if colliding := s.caseCollidingPath(id); colliding != "" {
			return errorWithProject(CodeProjectAlreadyExists, "project already exists", id)
		}
		created, err := s.persistUserLocked(id, def, nil)
		if err != nil {
			return err
		}
		snap = created
		s.emit(Event{Kind: EventProjectAdded, ProjectID: id, NewRevision: created.Revision})
		return nil
	})
	return snap, err
}

func (s *Store) Patch(ctx context.Context, req PatchRequest) (Snapshot, error) {
	var snap Snapshot
	err := s.withLock(ctx, func() error {
		if err := s.rebuildIndex(); err != nil {
			return err
		}
		rec := s.index[req.ProjectID]
		if rec == nil {
			return errorWithProject(CodeProjectNotFound, "project not found", req.ProjectID)
		}
		if rec.invalid || rec.user == nil {
			return &Error{Code: CodeInvalidDefinition, Message: "malformed sources cannot be patched", ProjectID: req.ProjectID, Diagnostics: cloneDiagnostics(rec.diagnostics)}
		}
		if rec.sourceRevision != req.ExpectedSourceRevision {
			return &Error{
				Code:                   CodeRevisionConflict,
				Message:                "project revision changed",
				ProjectID:              req.ProjectID,
				ExpectedSourceRevision: req.ExpectedSourceRevision,
				CurrentSourceRevision:  rec.sourceRevision,
			}
		}
		patched, err := applyUserPatch(rec.userBytes, req.Patch)
		if err != nil {
			return err
		}
		if patched.Name != rec.user.Name {
			return &Error{Code: CodeInvalidDefinition, Message: "patches may not change name", ProjectID: req.ProjectID, PathHint: "/name"}
		}
		oldRev := rec.revision
		created, err := s.persistUserLocked(req.ProjectID, patched, rec)
		if err != nil {
			return err
		}
		snap = created
		s.emit(Event{Kind: EventDefinitionChanged, ProjectID: req.ProjectID, OldRevision: oldRev, NewRevision: created.Revision})
		return nil
	})
	return snap, err
}

func (s *Store) Delete(ctx context.Context, req DeleteRequest) error {
	return s.withLock(ctx, func() error {
		if err := s.rebuildIndex(); err != nil {
			return err
		}
		rec := s.index[req.ProjectID]
		if rec == nil {
			return errorWithProject(CodeProjectNotFound, "project not found", req.ProjectID)
		}
		hasSrc := req.ExpectedSourceRevision != ""
		hasRaw := req.ExpectedRawSourceRevision != ""
		if hasSrc == hasRaw {
			return &Error{Code: CodeInvalidDefinition, Message: "exactly one CAS token is required", ProjectID: req.ProjectID}
		}
		if rec.invalid {
			if !hasRaw || rec.rawSourceRevision != req.ExpectedRawSourceRevision {
				return &Error{
					Code:                   CodeRevisionConflict,
					Message:                "project revision changed",
					ProjectID:              req.ProjectID,
					ExpectedSourceRevision: req.ExpectedRawSourceRevision,
					CurrentSourceRevision:  rec.rawSourceRevision,
				}
			}
		} else if rec.sourceRevision != req.ExpectedSourceRevision {
			return &Error{
				Code:                   CodeRevisionConflict,
				Message:                "project revision changed",
				ProjectID:              req.ProjectID,
				ExpectedSourceRevision: req.ExpectedSourceRevision,
				CurrentSourceRevision:  rec.sourceRevision,
			}
		}
		if err := os.Remove(rec.path); err != nil && !os.IsNotExist(err) {
			return &Error{Code: CodeInternalError, Message: err.Error(), ProjectID: req.ProjectID}
		}
		delete(s.index, req.ProjectID)
		delete(s.projects, string(req.ProjectID))
		s.emit(Event{Kind: EventProjectRemoved, ProjectID: req.ProjectID})
		return nil
	})
}

func (s *Store) CreateCompatibility(ctx context.Context, req CreateRequest) (PersistedUserDefinition, Snapshot, error) {
	snap, err := s.Create(ctx, req)
	if err != nil {
		return PersistedUserDefinition{}, Snapshot{}, err
	}
	return PersistedUserDefinition{Definition: *cloneDefinition(&req.Definition), Bytes: append([]byte(nil), snap.UserBytes...)}, snap, nil
}

func (s *Store) PatchCompatibility(ctx context.Context, id ProjectID, patch json.RawMessage) (Snapshot, error) {
	var snap Snapshot
	err := s.withLock(ctx, func() error {
		if err := s.rebuildIndex(); err != nil {
			return err
		}
		rec := s.index[id]
		if rec == nil || rec.user == nil {
			return errorWithProject(CodeProjectNotFound, "project not found", id)
		}
		patched, err := applyUserPatch(rec.userBytes, patch)
		if err != nil {
			return err
		}
		oldRev := rec.revision
		created, err := s.persistUserLocked(id, patched, rec)
		if err != nil {
			return err
		}
		snap = created
		s.emit(Event{Kind: EventDefinitionChanged, ProjectID: id, OldRevision: oldRev, NewRevision: created.Revision})
		return nil
	})
	return snap, err
}

func (s *Store) DeleteCompatibility(ctx context.Context, id ProjectID) error {
	return s.withLock(ctx, func() error {
		if err := s.rebuildIndex(); err != nil {
			return err
		}
		rec := s.index[id]
		if rec == nil {
			return errorWithProject(CodeProjectNotFound, "project not found", id)
		}
		if err := os.Remove(rec.path); err != nil && !os.IsNotExist(err) {
			return &Error{Code: CodeInternalError, Message: err.Error(), ProjectID: id}
		}
		delete(s.index, id)
		delete(s.projects, string(id))
		s.emit(Event{Kind: EventProjectRemoved, ProjectID: id})
		return nil
	})
}

func (s *Store) caseCollidingPath(id ProjectID) string {
	want := strings.ToLower(projectFileName(id))
	entries, err := os.ReadDir(s.projectsDir())
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if strings.ToLower(e.Name()) == want && e.Name() != projectFileName(id) {
			return filepath.Join(s.projectsDir(), e.Name())
		}
	}
	return ""
}

type revisionEnvelope struct {
	ProjectID  ProjectID  `json:"projectId"`
	Revision   Revision   `json:"revision"`
	Definition Definition `json:"definition"`
}

func (s *Store) persistUserLocked(id ProjectID, user *Definition, prev *catalogRecord) (Snapshot, error) {
	if err := user.Validate(); err != nil {
		return Snapshot{}, &Error{Code: CodeInvalidDefinition, Message: valueFreeValidationMessage(err), ProjectID: id}
	}
	effective, _, diags := s.mergeEffective(user, "", filepath.Join(s.projectsDir(), projectFileName(id)))
	if hasError(diags) {
		return Snapshot{}, &Error{Code: CodeInvalidDefinition, Message: "project definition is invalid", ProjectID: id, Diagnostics: diags}
	}
	rev, _, err := HashUserAndEffective(user, effective)
	if err != nil {
		return Snapshot{}, &Error{Code: CodeInternalError, Message: err.Error(), ProjectID: id}
	}
	if err := s.archiveRevision(id, rev, effective); err != nil {
		return Snapshot{}, err
	}
	data, err := json.MarshalIndent(user, "", "  ")
	if err != nil {
		return Snapshot{}, &Error{Code: CodeInternalError, Message: err.Error(), ProjectID: id}
	}
	if err := atomicWriteFile(filepath.Join(s.projectsDir(), projectFileName(id)), data, 0600); err != nil {
		return Snapshot{}, &Error{Code: CodeInternalError, Message: err.Error(), ProjectID: id}
	}
	if err := s.rebuildIndex(); err != nil {
		return Snapshot{}, err
	}
	rec := s.index[id]
	snap, err := s.snapshotFromRecord(rec, "")
	if err != nil {
		return Snapshot{}, err
	}
	_ = prev
	return snap, nil
}

func (s *Store) archiveRevision(id ProjectID, rev Revision, def *Definition) error {
	if err := os.MkdirAll(filepath.Join(s.revisionsDir(), string(id)), 0700); err != nil {
		return &Error{Code: CodeInternalError, Message: err.Error(), ProjectID: id}
	}
	path := filepath.Join(s.revisionsDir(), string(id), rev.DigestHex()+".json")
	env := revisionEnvelope{ProjectID: id, Revision: rev, Definition: *cloneDefinition(def)}
	data, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return &Error{Code: CodeInternalError, Message: err.Error(), ProjectID: id}
	}
	if existing, err := os.ReadFile(path); err == nil {
		if !bytes.Equal(bytes.TrimSpace(existing), bytes.TrimSpace(data)) {
			var old revisionEnvelope
			if json.Unmarshal(existing, &old) == nil {
				oldCanon, _ := CanonicalJSON(old.Definition)
				newCanon, _ := CanonicalJSON(env.Definition)
				if !bytes.Equal(oldCanon, newCanon) {
					return &Error{Code: CodeInternalError, Message: "revision archive integrity conflict", ProjectID: id}
				}
			}
		}
		return nil
	}
	return atomicWriteFile(path, data, 0600)
}

func applyUserPatch(userBytes, patch json.RawMessage) (*Definition, error) {
	var baseMap map[string]any
	if err := json.Unmarshal(userBytes, &baseMap); err != nil {
		return nil, &Error{Code: CodeInvalidDefinition, Message: "malformed sources cannot be patched"}
	}
	var patchMap map[string]any
	if err := json.Unmarshal(patch, &patchMap); err != nil {
		return nil, &Error{Code: CodeInvalidDefinition, Message: "invalid patch"}
	}
	if _, ok := patchMap["name"]; ok {
		return nil, &Error{Code: CodeInvalidDefinition, Message: "patches may not change name"}
	}
	merged := jsonMergePatch(baseMap, patchMap)
	raw, err := json.Marshal(merged)
	if err != nil {
		return nil, &Error{Code: CodeInternalError, Message: err.Error()}
	}
	var def Definition
	if err := json.Unmarshal(raw, &def); err != nil {
		return nil, &Error{Code: CodeInvalidDefinition, Message: "invalid project definition after patch"}
	}
	if err := def.Validate(); err != nil {
		return nil, &Error{Code: CodeInvalidDefinition, Message: valueFreeValidationMessage(err)}
	}
	return &def, nil
}

func atomicWriteFile(path string, data []byte, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, mode); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
