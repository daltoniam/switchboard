package project

import (
	"bytes"
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func gitCommonDir(ctx context.Context, root string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, "git", "-C", root, "rev-parse", "--path-format=absolute", "--git-common-dir") // #nosec G204 -- git binary is fixed; root is a canonical existing directory // #nosec G204 -- fixed git argv; root is a validated absolute directory
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}
	out := strings.TrimSpace(stdout.String())
	if out == "" {
		return "", &Error{Code: CodeInvalidRoot, Message: "git common directory is empty"}
	}
	abs, err := filepath.EvalSymlinks(filepath.Clean(out))
	if err != nil {
		abs = filepath.Clean(out)
	}
	return abs, nil
}

func (s *Store) matchProjectByRoot(ctx context.Context, root string) (ProjectID, *Error) {
	matches := make([]ProjectID, 0, 1)
	rootCommon, err := gitCommonDir(ctx, root)
	if err != nil {
		return "", &Error{Code: CodeInvalidRoot, Message: "rootUri is not a git worktree"}
	}
	for id, rec := range s.index {
		if rec == nil || rec.user == nil || rec.invalid {
			continue
		}
		repo := rec.user.ResolvedRepo()
		if repo == "" {
			continue
		}
		repoCommon, err := gitCommonDir(ctx, repo)
		if err != nil {
			continue
		}
		if repoCommon == rootCommon {
			matches = append(matches, id)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return "", &Error{Code: CodeProjectNotFound, Message: "no project matches rootUri"}
	default:
		return "", &Error{Code: CodeAmbiguousProject, Message: "multiple projects share this git common directory"}
	}
}
