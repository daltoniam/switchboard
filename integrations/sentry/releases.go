package sentry

import (
	"context"
	"fmt"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

func listReleases(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	query := r.Str("query")
	cursor := r.Str("cursor")
	project := r.Str("project")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	params := map[string]string{
		"query":  query,
		"cursor": cursor,
	}
	if project != "" {
		params["project"] = project
	}
	q := queryEncode(params)
	data, err := s.get(ctx, "/organizations/%s/releases/%s", s.org(args), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getRelease(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	version := r.Str("version")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.get(ctx, "/organizations/%s/releases/%s/", s.org(args), version)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createRelease(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	version := r.Str("version")
	projects := r.Str("projects")
	ref := r.Str("ref")
	u := r.Str("url")
	dateReleased := r.Str("dateReleased")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{
		"version":  version,
		"projects": strings.Split(projects, ","),
	}
	if ref != "" {
		body["ref"] = ref
	}
	if u != "" {
		body["url"] = u
	}
	if dateReleased != "" {
		body["dateReleased"] = dateReleased
	}
	path := fmt.Sprintf("/organizations/%s/releases/", s.org(args))
	data, err := s.post(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteRelease(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	version := r.Str("version")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.del(ctx, "/organizations/%s/releases/%s/", s.org(args), version)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listReleaseCommits(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	version := r.Str("version")
	cursor := r.Str("cursor")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(map[string]string{"cursor": cursor})
	data, err := s.get(ctx, "/organizations/%s/releases/%s/commits/%s", s.org(args), version, q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listReleaseDeploys(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	version := r.Str("version")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.get(ctx, "/organizations/%s/releases/%s/deploys/", s.org(args), version)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createDeploy(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	version := r.Str("version")
	environment := r.Str("environment")
	name := r.Str("name")
	u := r.Str("url")
	dateStarted := r.Str("dateStarted")
	dateFinished := r.Str("dateFinished")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]string{
		"environment": environment,
	}
	if name != "" {
		body["name"] = name
	}
	if u != "" {
		body["url"] = u
	}
	if dateStarted != "" {
		body["dateStarted"] = dateStarted
	}
	if dateFinished != "" {
		body["dateFinished"] = dateFinished
	}
	path := fmt.Sprintf("/organizations/%s/releases/%s/deploys/", s.org(args), version)
	data, err := s.post(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listReleaseFiles(ctx context.Context, s *sentry, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	version := r.Str("version")
	cursor := r.Str("cursor")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(map[string]string{"cursor": cursor})
	data, err := s.get(ctx, "/organizations/%s/releases/%s/files/%s", s.org(args), version, q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
