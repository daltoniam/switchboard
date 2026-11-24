package openproject

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	mcp "github.com/daltoniam/switchboard"
)

type retryWaitTransport func(*http.Request) (*http.Response, error)

func (transport retryWaitTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

func TestRetryWaitHonorsCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		requests := 0
		integration := &openproject{
			baseURL:     "https://openproject.example.com",
			workspaceID: 7,
			apiToken:    "secret-token",
			client: &http.Client{Transport: retryWaitTransport(func(*http.Request) (*http.Response, error) {
				requests++
				return &http.Response{
					StatusCode: http.StatusTooManyRequests,
					Header:     http.Header{"Retry-After": {"2"}},
					Body:       http.NoBody,
				}, nil
			})},
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		var result *mcp.ToolResult
		var err error
		done := make(chan struct{})
		go func() {
			result, err = integration.Execute(ctx, "openproject_list_work_packages", nil)
			close(done)
		}()
		synctest.Wait()
		if requests != 1 {
			t.Fatalf("requests before cancellation = %d, want 1", requests)
		}
		select {
		case <-done:
			t.Fatal("request completed before retry wait cancellation")
		default:
		}
		started := time.Now()
		cancel()
		<-done
		if elapsed := time.Since(started); elapsed != 0 {
			t.Fatalf("retry wait advanced time by %s after cancellation", elapsed)
		}
		if err != nil || result == nil || !result.IsError || !strings.Contains(result.Data, "request canceled during rate-limit wait") {
			t.Fatalf("expected retry-wait cancellation, got result=%+v err=%v", result, err)
		}
		if requests != 1 {
			t.Fatalf("requests after cancellation = %d, want 1", requests)
		}
		assertSafe(t, result.Data)
	})
}
