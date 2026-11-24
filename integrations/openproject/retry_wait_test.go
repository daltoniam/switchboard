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

func TestRetryBudgetSpansAttempts(t *testing.T) {
	for _, testCase := range []struct {
		name          string
		execute       bool
		responseDelay time.Duration
		wantError     string
	}{
		{"execute deadline during response", true, 6 * time.Second, "context deadline"},
		{"execute deadline during backoff", true, 3500 * time.Millisecond, "request canceled during rate-limit wait"},
		{"get deadline during response", false, 6 * time.Second, "context deadline"},
		{"get deadline during backoff", false, 3500 * time.Millisecond, "request canceled during rate-limit wait"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				started := time.Now()
				var requestTimes []time.Duration
				var deadlines []time.Time
				integration := &openproject{
					baseURL:     "https://openproject.example.com",
					workspaceID: 7,
					apiToken:    "secret-token",
					client: &http.Client{Transport: retryWaitTransport(func(request *http.Request) (*http.Response, error) {
						requestTimes = append(requestTimes, time.Since(started))
						deadline, _ := request.Context().Deadline()
						deadlines = append(deadlines, deadline)
						timer := time.NewTimer(testCase.responseDelay)
						defer timer.Stop()
						select {
						case <-request.Context().Done():
							return nil, request.Context().Err()
						case <-timer.C:
							return &http.Response{
								StatusCode: http.StatusTooManyRequests,
								Header:     http.Header{"Retry-After": {"2"}},
								Body:       http.NoBody,
							}, nil
						}
					})},
				}
				var message string
				if testCase.execute {
					result, err := integration.Execute(context.Background(), "openproject_list_work_packages", nil)
					if err != nil || result == nil || !result.IsError {
						t.Fatalf("expected overall deadline error, got result=%+v err=%v", result, err)
					}
					message = result.Data
				} else {
					var target any
					err := integration.get(context.Background(), "/api/v3/workspaces/7/work_packages", nil, &target)
					if err == nil {
						t.Fatal("expected overall deadline error")
					}
					message = err.Error()
				}
				if elapsed := time.Since(started); elapsed != requestTimeout {
					t.Fatalf("operation lasted %s, want shared budget %s", elapsed, requestTimeout)
				}
				if len(requestTimes) != 2 || requestTimes[0] != 0 || requestTimes[1] != testCase.responseDelay+2*time.Second {
					t.Fatalf("request start times = %v, want two attempts separated by response and backoff", requestTimes)
				}
				for attempt, deadline := range deadlines {
					if !deadline.Equal(started.Add(requestTimeout)) {
						t.Errorf("attempt %d deadline = %s, want operation-wide deadline %s", attempt+1, deadline, started.Add(requestTimeout))
					}
				}
				if !strings.Contains(message, testCase.wantError) {
					t.Fatalf("error = %s, want %q", message, testCase.wantError)
				}
				assertSafe(t, message)
			})
		})
	}
}

func TestRetryAfterInvalidHeadersDoNotScheduleWait(t *testing.T) {
	for _, header := range []string{"", "-1", "1.5", "PRIVATE", "999999999999999999999999999", "60", "future HTTP date"} {
		t.Run(header, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				if header == "future HTTP date" {
					header = time.Now().Add(time.Hour).UTC().Format(http.TimeFormat)
				}
				requests := 0
				const transportLatency = 1500 * time.Millisecond
				integration := &openproject{
					baseURL:     "https://openproject.example.com",
					workspaceID: 7,
					apiToken:    "secret-token",
					client: &http.Client{Transport: retryWaitTransport(func(*http.Request) (*http.Response, error) {
						requests++
						time.Sleep(transportLatency)
						return &http.Response{
							StatusCode: http.StatusTooManyRequests,
							Header:     http.Header{"Retry-After": {header}},
							Body:       http.NoBody,
						}, nil
					})},
				}
				started := time.Now()
				result := execute(t, integration, "openproject_list_work_packages", nil)
				if !result.IsError || !strings.Contains(result.Data, "no bounded valid Retry-After") || requests != 1 {
					t.Fatalf("invalid Retry-After must fail without retry: result=%+v requests=%d", result, requests)
				}
				if elapsed := time.Since(started); elapsed != transportLatency {
					t.Fatalf("invalid Retry-After scheduled %s beyond transport latency", elapsed-transportLatency)
				}
				assertSafe(t, result.Data)
			})
		})
	}
}
