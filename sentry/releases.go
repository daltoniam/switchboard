package sentry

import (
	"context"
	"fmt"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

func listReleases(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	params := map[string]string{
		"query":  argStr(args, "query"),
		"cursor": argStr(args, "cursor"),
	}
	if v := argStr(args, "project"); v != "" {
		params["project"] = v
	}
	q := queryEncode(params)
	data, err := s.get(ctx, "/organizations/%s/releases/%s", s.org(args), q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getRelease(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	data, err := s.get(ctx, "/organizations/%s/releases/%s/", s.org(args), argStr(args, "version"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func createRelease(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{
		"version":  argStr(args, "version"),
		"projects": strings.Split(argStr(args, "projects"), ","),
	}
	if v := argStr(args, "ref"); v != "" {
		body["ref"] = v
	}
	if v := argStr(args, "url"); v != "" {
		body["url"] = v
	}
	if v := argStr(args, "dateReleased"); v != "" {
		body["dateReleased"] = v
	}
	path := fmt.Sprintf("/organizations/%s/releases/", s.org(args))
	data, err := s.post(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func deleteRelease(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	data, err := s.del(ctx, "/organizations/%s/releases/%s/", s.org(args), argStr(args, "version"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func listReleaseCommits(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	q := queryEncode(map[string]string{"cursor": argStr(args, "cursor")})
	data, err := s.get(ctx, "/organizations/%s/releases/%s/commits/%s", s.org(args), argStr(args, "version"), q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func listReleaseDeploys(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	data, err := s.get(ctx, "/organizations/%s/releases/%s/deploys/", s.org(args), argStr(args, "version"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func createDeploy(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]string{
		"environment": argStr(args, "environment"),
	}
	if v := argStr(args, "name"); v != "" {
		body["name"] = v
	}
	if v := argStr(args, "url"); v != "" {
		body["url"] = v
	}
	if v := argStr(args, "dateStarted"); v != "" {
		body["dateStarted"] = v
	}
	if v := argStr(args, "dateFinished"); v != "" {
		body["dateFinished"] = v
	}
	path := fmt.Sprintf("/organizations/%s/releases/%s/deploys/", s.org(args), argStr(args, "version"))
	data, err := s.post(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func listReleaseFiles(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	q := queryEncode(map[string]string{"cursor": argStr(args, "cursor")})
	data, err := s.get(ctx, "/organizations/%s/releases/%s/files/%s", s.org(args), argStr(args, "version"), q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}
