package linear

import (
	"context"

	mcp "github.com/daltoniam/switchboard"
)

// ── Teams ─────────────────────────────────────────────────────────

func listTeams(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	data, err := l.gql(ctx, `query($first: Int) {
		teams(first: $first) {
			nodes {
				id name key description private
				members { nodes { id name email } }
				states { nodes { id name type color position } }
				labels { nodes { id name color } }
				cycles(first: 3, orderBy: createdAt) { nodes { id name number startsAt endsAt } }
			}
		}
	}`, map[string]any{"first": optInt(args, "first", 50)})
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getTeam(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	id := argStr(args, "id")
	data, err := l.gql(ctx, `query($id: String!) {
		team(id: $id) {
			id name key description private
			members { nodes { id name email } }
			states { nodes { id name type color position } }
			labels { nodes { id name color } }
			cycles(first: 5, orderBy: createdAt) { nodes { id name number startsAt endsAt } }
			projects { nodes { id name state } }
		}
	}`, map[string]any{"id": id})
	if err != nil {
		teamID, resolveErr := l.resolveTeamID(ctx, id)
		if resolveErr != nil {
			return errResult(err)
		}
		data, err = l.gql(ctx, `query($id: String!) {
			team(id: $id) {
				id name key description private
				members { nodes { id name email } }
				states { nodes { id name type color position } }
				labels { nodes { id name color } }
				cycles(first: 5, orderBy: createdAt) { nodes { id name number startsAt endsAt } }
				projects { nodes { id name state } }
			}
		}`, map[string]any{"id": teamID})
		if err != nil {
			return errResult(err)
		}
	}
	return rawResult(data)
}

// ── Users ─────────────────────────────────────────────────────────

func viewer(ctx context.Context, l *linear, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := l.gql(ctx, `{
		viewer {
			id name displayName email admin active url
			organization { id name urlKey }
			teams { nodes { id name key } }
		}
	}`, nil)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func listUsers(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	data, err := l.gql(ctx, `query($first: Int) {
		users(first: $first) {
			nodes {
				id name displayName email admin active guest
				teams { nodes { id name key } }
			}
		}
	}`, map[string]any{"first": optInt(args, "first", 50)})
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getUser(ctx context.Context, l *linear, args map[string]any) (*mcp.ToolResult, error) {
	data, err := l.gql(ctx, `query($id: String!) {
		user(id: $id) {
			id name displayName email admin active url
			teams { nodes { id name key } }
			assignedIssues(first: 10, orderBy: updatedAt) {
				nodes { id identifier title state { name } }
			}
		}
	}`, map[string]any{"id": argStr(args, "id")})
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}
