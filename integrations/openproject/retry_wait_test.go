package openproject

import (
	"context"
	"io"
	"net/http"
	"reflect"
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

func TestRetryAfterSuccessfulWaitMatchesHeader(t *testing.T) {
	for _, testCase := range []struct {
		name        string
		header      string
		clockOffset time.Duration
		wantDelay   time.Duration
	}{
		{"zero seconds", "0", 0, 0},
		{"one second", "1", 0, time.Second},
		{"maximum seconds", "2", 0, 2 * time.Second},
		{"future date whole second", "future HTTP date", 0, 2 * time.Second},
		{"future date fractional second", "future HTTP date", 250 * time.Millisecond, 1750 * time.Millisecond},
		{"past date", "past HTTP date", 250 * time.Millisecond, 0},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				time.Sleep(time.Until(time.Now().Truncate(time.Second).Add(time.Second)) + testCase.clockOffset)
				started := time.Now()
				header := testCase.header
				switch header {
				case "future HTTP date":
					header = started.Add(2 * time.Second).UTC().Format(http.TimeFormat)
				case "past HTTP date":
					header = started.Add(-time.Second).UTC().Format(http.TimeFormat)
				}
				var requestTimes []time.Duration
				integration := &openproject{
					baseURL:     "https://openproject.example.com/openproject",
					workspaceID: 7,
					apiToken:    "secret-token",
					client: &http.Client{Transport: retryWaitTransport(func(*http.Request) (*http.Response, error) {
						requestTimes = append(requestTimes, time.Since(started))
						if len(requestTimes) == 1 {
							return &http.Response{
								StatusCode: http.StatusTooManyRequests,
								Header:     http.Header{"Retry-After": {header}},
								Body:       http.NoBody,
							}, nil
						}
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     http.Header{"Content-Type": {"application/hal+json"}},
							Body:       io.NopCloser(strings.NewReader(collectionFixture(1, 1, 1, 20, workPackageFixture))),
						}, nil
					})},
				}
				result := execute(t, integration, "openproject_list_work_packages", nil)
				if got, want := successJSON(t, result), listJSON(1, 1, 1, 20, 1, 0, nil, summaryJSON()); !reflect.DeepEqual(got, want) {
					t.Fatalf("retried collection = %#v, want %#v", got, want)
				}
				if want := []time.Duration{0, testCase.wantDelay}; !reflect.DeepEqual(requestTimes, want) {
					t.Fatalf("request start times = %v, want %v", requestTimes, want)
				}
				if elapsed := time.Since(started); elapsed != testCase.wantDelay {
					t.Fatalf("successful retry lasted %s, want %s", elapsed, testCase.wantDelay)
				}
				assertSafe(t, result.Data)
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
