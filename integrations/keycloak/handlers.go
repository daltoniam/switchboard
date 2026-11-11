package keycloak

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"time"
	"unicode/utf8"

	mcp "github.com/daltoniam/switchboard"
)

type handlerFunc func(context.Context, *connection, map[string]any) (*mcp.ToolResult, error)

var dispatch = map[mcp.ToolName]handlerFunc{
	"keycloak_list_users":  listUsers,
	"keycloak_get_user":    getUser,
	"keycloak_list_groups": listGroups,
	"keycloak_list_events": listEvents,
}

type user struct {
	ID               string `json:"id"`
	Username         string `json:"username"`
	Enabled          *bool  `json:"enabled,omitempty"`
	Email            string `json:"email,omitempty"`
	EmailVerified    *bool  `json:"emailVerified,omitempty"`
	FirstName        string `json:"firstName,omitempty"`
	LastName         string `json:"lastName,omitempty"`
	CreatedTimestamp *int64 `json:"createdTimestamp,omitempty"`
}

type group struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Path          string `json:"path"`
	ParentID      string `json:"parentId,omitempty"`
	SubGroupCount *int   `json:"subGroupCount,omitempty"`
}

type event struct {
	Time     int64  `json:"time"`
	Type     string `json:"type"`
	UserID   string `json:"userId,omitempty"`
	ClientID string `json:"clientId,omitempty"`
	Error    string `json:"error,omitempty"`
}

type page[T any] struct {
	Items           []T    `json:"items"`
	First           int    `json:"first"`
	Max             int    `json:"max"`
	ReturnedCount   int    `json:"returned_count"`
	HasMorePossible bool   `json:"has_more_possible"`
	EventsEnabled   *bool  `json:"events_enabled,omitempty"`
	DateFrom        string `json:"date_from,omitempty"`
	DateTo          string `json:"date_to,omitempty"`
}

func pagination(firstValue, maxValue string) (int, int, error) {
	if firstValue == "" {
		firstValue = "0"
	}
	if maxValue == "" {
		maxValue = "20"
	}
	first, firstErr := strconv.Atoi(firstValue)
	maximum, maxErr := strconv.Atoi(maxValue)
	if firstErr != nil || maxErr != nil || first < 0 || first > 1000000 || maximum < 1 || maximum > 100 {
		return 0, 0, fmt.Errorf("keycloak: first must be an integer 0-1000000 and max an integer 1-100")
	}
	return first, maximum, nil
}

func listQuery(args map[string]any) (url.Values, int, int, error) {
	reader := mcp.NewArgs(args)
	search := reader.Str("search")
	firstValue := reader.Str("first")
	maxValue := reader.Str("max")
	if err := reader.Err(); err != nil {
		return nil, 0, 0, fmt.Errorf("keycloak: invalid list parameter type")
	}
	first, maximum, err := pagination(firstValue, maxValue)
	if err != nil {
		return nil, 0, 0, err
	}
	if utf8.RuneCountInString(search) > 512 {
		return nil, 0, 0, fmt.Errorf("keycloak: search exceeds 512 characters")
	}
	query := url.Values{"first": {strconv.Itoa(first)}, "max": {strconv.Itoa(maximum)}, "briefRepresentation": {"true"}}
	if search != "" {
		query.Set("search", search)
	}
	return query, first, maximum, nil
}

func listUsers(ctx context.Context, current *connection, args map[string]any) (*mcp.ToolResult, error) {
	query, first, maximum, err := listQuery(args)
	if err != nil {
		return nil, err
	}
	var users []user
	if err := current.read(ctx, "/users", query, &users); err != nil {
		return nil, err
	}
	if users == nil || len(users) > maximum {
		return nil, fmt.Errorf("keycloak: invalid users page")
	}
	return mcp.JSONResult(page[user]{Items: users, First: first, Max: maximum, ReturnedCount: len(users), HasMorePossible: len(users) == maximum})
}

func getUser(ctx context.Context, current *connection, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgStr(args, "user_id")
	if err != nil || !safeSegment(id) {
		return nil, fmt.Errorf("keycloak: user_id must be one literal nonempty ID")
	}
	var profile user
	if err := current.read(ctx, "/users/"+id, nil, &profile); err != nil {
		return nil, err
	}
	if profile.ID != id {
		return nil, fmt.Errorf("keycloak: returned user ID does not match requested user")
	}
	return mcp.JSONResult(profile)
}

func listGroups(ctx context.Context, current *connection, args map[string]any) (*mcp.ToolResult, error) {
	query, first, maximum, err := listQuery(args)
	if err != nil {
		return nil, err
	}
	query.Set("populateHierarchy", "false")
	var groups []group
	if err := current.read(ctx, "/groups", query, &groups); err != nil {
		return nil, err
	}
	if groups == nil || len(groups) > maximum {
		return nil, fmt.Errorf("keycloak: invalid groups page")
	}
	return mcp.JSONResult(page[group]{Items: groups, First: first, Max: maximum, ReturnedCount: len(groups), HasMorePossible: len(groups) == maximum})
}

func listEvents(ctx context.Context, current *connection, args map[string]any) (*mcp.ToolResult, error) {
	reader := mcp.NewArgs(args)
	dateFrom := reader.Str("date_from")
	dateTo := reader.Str("date_to")
	eventType := reader.Str("type")
	userID := reader.Str("user")
	firstValue := reader.Str("first")
	maxValue := reader.Str("max")
	if err := reader.Err(); err != nil {
		return nil, fmt.Errorf("keycloak: invalid event parameter type")
	}
	first, maximum, err := pagination(firstValue, maxValue)
	if err != nil {
		return nil, err
	}
	from, fromErr := time.Parse(time.DateOnly, dateFrom)
	to, toErr := time.Parse(time.DateOnly, dateTo)
	if fromErr != nil || toErr != nil || to.Before(from) || to.Sub(from) >= 7*24*time.Hour {
		return nil, fmt.Errorf("keycloak: date_from and date_to must be YYYY-MM-DD, ordered, and span at most seven inclusive UTC calendar days")
	}
	if eventType != "" && eventType != "LOGIN" && eventType != "LOGIN_ERROR" {
		return nil, fmt.Errorf("keycloak: type must be LOGIN or LOGIN_ERROR")
	}
	if userID != "" && !safeSegment(userID) {
		return nil, fmt.Errorf("keycloak: user must be a literal user ID")
	}
	var config struct {
		EventsEnabled *bool `json:"eventsEnabled"`
	}
	if err := current.read(ctx, "/events/config", nil, &config); err != nil {
		return nil, err
	}
	if config.EventsEnabled == nil {
		return nil, fmt.Errorf("keycloak: event persistence state is unavailable")
	}
	if !*config.EventsEnabled {
		return nil, fmt.Errorf("keycloak: events not enabled; ask the realm administrator to review event persistence")
	}
	query := url.Values{"first": {strconv.Itoa(first)}, "max": {strconv.Itoa(maximum)}, "dateFrom": {dateFrom}, "dateTo": {dateTo}}
	if userID != "" {
		query.Set("user", userID)
	}
	if eventType == "" {
		query["type"] = []string{"LOGIN", "LOGIN_ERROR"}
	} else {
		query.Set("type", eventType)
	}
	var events []event
	if err := current.read(ctx, "/events", query, &events); err != nil {
		return nil, err
	}
	if events == nil || len(events) > maximum {
		return nil, fmt.Errorf("keycloak: invalid events page")
	}
	for _, item := range events {
		if item.Type != "LOGIN" && item.Type != "LOGIN_ERROR" || eventType != "" && item.Type != eventType || userID != "" && item.UserID != userID || item.Time < from.UnixMilli() || item.Time >= to.AddDate(0, 0, 1).UnixMilli() {
			return nil, fmt.Errorf("keycloak: event response falls outside requested login filters")
		}
	}
	return mcp.JSONResult(page[event]{Items: events, First: first, Max: maximum, ReturnedCount: len(events), HasMorePossible: len(events) == maximum, EventsEnabled: config.EventsEnabled, DateFrom: dateFrom, DateTo: dateTo})
}
