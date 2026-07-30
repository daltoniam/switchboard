package rwx

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

func (r *rwx) apiGetJSON(ctx context.Context, apiPath string, query url.Values, out any) error {
	return r.apiDoJSON(ctx, http.MethodGet, apiPath, query, nil, out)
}

func (r *rwx) apiDoJSON(ctx context.Context, method, apiPath string, query url.Values, body io.Reader, out any) error {
	req, err := r.apiRequest(ctx, method, apiPath, query, body)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if err := rwxAPIError(resp); err != nil {
		return err
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxResponseSize))
		return nil
	}
	return json.NewDecoder(io.LimitReader(resp.Body, maxResponseSize)).Decode(out)
}

func (r *rwx) apiGetText(ctx context.Context, apiPath string, query url.Values) (string, error) {
	req, err := r.apiRequest(ctx, http.MethodGet, apiPath, query, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "text/plain")

	resp, err := r.client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if err := rwxAPIError(resp); err != nil {
		return "", err
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (r *rwx) apiRequest(ctx context.Context, method, apiPath string, query url.Values, body io.Reader) (*http.Request, error) {
	apiURL := strings.TrimRight(r.baseURL, "/") + "/" + strings.TrimLeft(apiPath, "/")
	u, err := url.Parse(apiURL)
	if err != nil {
		return nil, err
	}
	if query != nil {
		u.RawQuery = query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+r.accessToken)
	return req, nil
}

func rwxAPIError(resp *http.Response) error {
	if resp.StatusCode < 400 {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	err := fmt.Errorf("RWX API error (%d): %s", resp.StatusCode, string(body))
	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		re := &mcp.RetryableError{StatusCode: resp.StatusCode, Err: err}
		re.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return re
	}
	return err
}

type rwxNestedStatus struct {
	Result        string `json:"result"`
	Execution     string `json:"execution"`
	AbortedStatus string `json:"aborted_status"`
}

type rwxStatusResult struct {
	RunID       string          `json:"run_id"`
	RunIDCamel  string          `json:"RunID"`
	TaskID      string          `json:"task_id"`
	TaskIDCamel string          `json:"TaskID"`
	RunStatus   rwxNestedStatus `json:"run_status"`
	TaskStatus  rwxNestedStatus `json:"task_status"`
	Polling     struct {
		Completed bool `json:"completed"`
	} `json:"polling"`
	// Legacy flat fields — retained for backward compatibility with any
	// endpoint or fixture that still returns the un-nested shape.
	ExecutionStatus           string `json:"execution_status"`
	ExecutionStatusCamel      string `json:"ExecutionStatus"`
	ResultStatus              string `json:"result_status"`
	ResultStatusCamel         string `json:"ResultStatus"`
	Completed                 bool   `json:"completed"`
	CompletedCamel            bool   `json:"Completed"`
	ExecutionAbortedSubStatus string `json:"execution_aborted_sub_status"`
}

func (s rwxStatusResult) runID(fallback string) string {
	if s.RunID != "" {
		return s.RunID
	}
	if s.RunIDCamel != "" {
		return s.RunIDCamel
	}
	return fallback
}

func (s rwxStatusResult) taskID() string {
	if s.TaskID != "" {
		return s.TaskID
	}
	return s.TaskIDCamel
}

func (s rwxStatusResult) executionStatus() string {
	if s.TaskStatus.Execution != "" {
		return s.TaskStatus.Execution
	}
	if s.RunStatus.Execution != "" {
		return s.RunStatus.Execution
	}
	if s.ExecutionStatus != "" {
		return s.ExecutionStatus
	}
	return s.ExecutionStatusCamel
}

func (s rwxStatusResult) resultStatus() string {
	if s.TaskStatus.Result != "" {
		return s.TaskStatus.Result
	}
	if s.RunStatus.Result != "" {
		return s.RunStatus.Result
	}
	if s.ResultStatus != "" {
		return s.ResultStatus
	}
	return s.ResultStatusCamel
}

func (s rwxStatusResult) completed() bool {
	return s.Polling.Completed || s.Completed || s.CompletedCamel || s.executionStatus() == "finished"
}

func (s rwxStatusResult) abortedSubStatus() string {
	if s.TaskStatus.AbortedStatus != "" && s.TaskStatus.AbortedStatus != "not_applicable" {
		return s.TaskStatus.AbortedStatus
	}
	if s.RunStatus.AbortedStatus != "" && s.RunStatus.AbortedStatus != "not_applicable" {
		return s.RunStatus.AbortedStatus
	}
	return s.ExecutionAbortedSubStatus
}

type rwxLogDownload struct {
	URL      string `json:"url"`
	Token    string `json:"token"`
	Filename string `json:"filename"`
	Contents string `json:"contents"`
}

type rwxArtifactDownload struct {
	URL      string `json:"url"`
	Token    string `json:"token"`
	Filename string `json:"filename"`
	Key      string `json:"key"`
	TaskKey  string `json:"task_key"`
}

func (a rwxArtifactDownload) toMap(omitToken bool) map[string]any {
	item := map[string]any{
		"key":      a.Key,
		"filename": a.Filename,
		"url":      a.URL,
	}
	if a.TaskKey != "" {
		item["task_key"] = a.TaskKey
	}
	if !omitToken && a.Token != "" {
		item["token"] = a.Token
	}
	return item
}

func (r *rwx) downloadArtifact(ctx context.Context, artifact rwxArtifactDownload) (string, error) {
	if artifact.URL == "" {
		return "", fmt.Errorf("RWX API response did not include artifact download URL")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, artifact.URL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/octet-stream")

	resp, err := r.client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if err := rwxAPIError(resp); err != nil {
		return "", err
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (r *rwx) downloadLogArchive(ctx context.Context, request rwxLogDownload) ([]byte, error) {
	form := url.Values{}
	form.Set("token", request.Token)
	form.Set("filename", request.Filename)
	form.Set("contents", request.Contents)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, request.URL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/octet-stream")

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if err := rwxAPIError(resp); err != nil {
		return nil, err
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
}

func extractTextFromArchive(data []byte) (string, error) {
	zr, err := zipReader(data)
	if err != nil {
		return string(data), nil
	}

	var contents []string
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if !isLogLikePath(f.Name) {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			continue
		}
		data, readErr := io.ReadAll(io.LimitReader(rc, maxResponseSize))
		_ = rc.Close()
		if readErr != nil {
			continue
		}
		if len(zr.File) == 1 {
			return string(data), nil
		}
		contents = append(contents, fmt.Sprintf("\n=== %s ===\n%s", f.Name, string(data)))
	}
	if len(contents) == 0 {
		return "", fmt.Errorf("no log files found in downloaded output")
	}
	return strings.Join(contents, "\n"), nil
}

func zipReader(data []byte) (*zip.Reader, error) {
	return zip.NewReader(bytes.NewReader(data), int64(len(data)))
}

func isLogLikePath(name string) bool {
	ext := strings.ToLower(path.Ext(name))
	return ext == ".log" || ext == ".txt" || ext == ""
}
