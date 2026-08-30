package project

import "context"

// matchProjectByRoot cannot associate worktrees without repository bindings.
func (s *Store) matchProjectByRoot(ctx context.Context, root string) (ProjectID, *Error) {
	_ = ctx
	_ = root
	return "", &Error{Code: CodeProjectNotFound, Message: "rootUri resolution requires an explicit projectId"}
}
