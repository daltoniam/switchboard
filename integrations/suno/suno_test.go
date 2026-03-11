package suno

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	i := New()
	require.NotNil(t, i)
	assert.Equal(t, "suno", i.Name())
}

func TestConfigure_Success(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"api_key": "test-key"})
	assert.NoError(t, err)
}

func TestConfigure_MissingAPIKey(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"api_key": ""})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "api_key is required")
}

func TestConfigure_CustomBaseURL(t *testing.T) {
	s := &suno{client: &http.Client{}, baseURL: "https://api.sunoapi.org"}
	err := s.Configure(context.Background(), mcp.Credentials{
		"api_key":  "test",
		"base_url": "https://custom.suno.com/",
	})
	assert.NoError(t, err)
	assert.Equal(t, "https://custom.suno.com", s.baseURL)
}

func TestTools(t *testing.T) {
	i := New()
	tools := i.Tools()
	assert.NotEmpty(t, tools)

	for _, tool := range tools {
		assert.NotEmpty(t, tool.Name, "tool has empty name")
		assert.NotEmpty(t, tool.Description, "tool %s has empty description", tool.Name)
	}
}

func TestTools_AllHaveSunoPrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, tool.Name, "suno_", "tool %s missing suno_ prefix", tool.Name)
	}
}

func TestTools_NoDuplicateNames(t *testing.T) {
	i := New()
	seen := make(map[string]bool)
	for _, tool := range i.Tools() {
		assert.False(t, seen[tool.Name], "duplicate tool name: %s", tool.Name)
		seen[tool.Name] = true
	}
}

func TestExecute_UnknownTool(t *testing.T) {
	s := &suno{apiKey: "test", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := s.Execute(context.Background(), "suno_nonexistent", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "unknown tool")
}

func TestDispatchMap_AllToolsCovered(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		_, ok := dispatch[tool.Name]
		assert.True(t, ok, "tool %s has no dispatch handler", tool.Name)
	}
}

func TestDispatchMap_NoOrphanHandlers(t *testing.T) {
	i := New()
	toolNames := make(map[string]bool)
	for _, tool := range i.Tools() {
		toolNames[tool.Name] = true
	}
	for name := range dispatch {
		assert.True(t, toolNames[name], "dispatch handler %s has no tool definition", name)
	}
}

// --- HTTP helper tests ---

func TestDoRequest_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"msg":"success","data":{"taskId":"abc-123"}}`))
	}))
	defer ts.Close()

	s := &suno{apiKey: "test-key", client: ts.Client(), baseURL: ts.URL}
	data, err := s.get(context.Background(), "/api/v1/generate/credit")
	require.NoError(t, err)
	assert.Contains(t, string(data), "abc-123")
}

func TestDoRequest_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"code":401,"msg":"unauthorized"}`))
	}))
	defer ts.Close()

	s := &suno{apiKey: "bad-key", client: ts.Client(), baseURL: ts.URL}
	_, err := s.get(context.Background(), "/api/v1/generate/credit")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "suno API error (401)")
}

func TestDoRequest_204NoContent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(204)
	}))
	defer ts.Close()

	s := &suno{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	data, err := s.doRequest(context.Background(), "POST", "/test", nil)
	require.NoError(t, err)
	assert.Contains(t, string(data), "success")
}

func TestPost(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.NotEmpty(t, body)
		_, _ = w.Write([]byte(`{"code":200,"msg":"success","data":{"taskId":"task-1"}}`))
	}))
	defer ts.Close()

	s := &suno{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	data, err := s.post(context.Background(), "/test", map[string]string{"key": "val"})
	require.NoError(t, err)
	assert.Contains(t, string(data), "task-1")
}

// --- Result helper tests ---

func TestRawResult(t *testing.T) {
	data := json.RawMessage(`{"key":"value"}`)
	result, err := rawResult(data)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, `{"key":"value"}`, result.Data)
}

func TestErrResult(t *testing.T) {
	result, err := errResult(fmt.Errorf("test error"))
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Equal(t, "test error", result.Data)
}

// --- Argument helper tests ---

func TestArgStr(t *testing.T) {
	assert.Equal(t, "val", argStr(map[string]any{"k": "val"}, "k"))
	assert.Empty(t, argStr(map[string]any{}, "k"))
}

func TestArgInt(t *testing.T) {
	assert.Equal(t, 42, argInt(map[string]any{"n": float64(42)}, "n"))
	assert.Equal(t, 42, argInt(map[string]any{"n": 42}, "n"))
	assert.Equal(t, 42, argInt(map[string]any{"n": "42"}, "n"))
	assert.Equal(t, 0, argInt(map[string]any{}, "n"))
}

func TestArgBool(t *testing.T) {
	assert.True(t, argBool(map[string]any{"b": true}, "b"))
	assert.False(t, argBool(map[string]any{"b": false}, "b"))
	assert.True(t, argBool(map[string]any{"b": "true"}, "b"))
	assert.False(t, argBool(map[string]any{}, "b"))
}

func TestArgFloat(t *testing.T) {
	assert.InDelta(t, 0.65, argFloat(map[string]any{"f": 0.65}, "f"), 0.001)
	assert.InDelta(t, 42.0, argFloat(map[string]any{"f": 42}, "f"), 0.001)
	assert.InDelta(t, 1.5, argFloat(map[string]any{"f": "1.5"}, "f"), 0.001)
	assert.InDelta(t, 0.0, argFloat(map[string]any{}, "f"), 0.001)
}

// --- Handler integration tests ---

func TestGenerateMusic(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/v1/generate", r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "A peaceful folk song", body["prompt"])
		assert.Equal(t, true, body["customMode"])
		_, _ = w.Write([]byte(`{"code":200,"msg":"success","data":{"taskId":"gen-123"}}`))
	}))
	defer ts.Close()

	s := &suno{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	result, err := s.Execute(context.Background(), "suno_generate_music", map[string]any{
		"prompt": "A peaceful folk song",
		"style":  "folk",
		"title":  "Peaceful Song",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "gen-123")
}

func TestGetGeneration(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "gen-123", r.URL.Query().Get("taskId"))
		_, _ = w.Write([]byte(`{"code":200,"data":{"taskId":"gen-123","status":"SUCCESS","response":{"sunoData":[{"id":"audio-1","title":"Test Song"}]}}}`))
	}))
	defer ts.Close()

	s := &suno{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	result, err := s.Execute(context.Background(), "suno_get_generation", map[string]any{
		"task_id": "gen-123",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "SUCCESS")
	assert.Contains(t, result.Data, "Test Song")
}

func TestExtendMusic(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/v1/generate/extend", r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "audio-1", body["audioId"])
		_, _ = w.Write([]byte(`{"code":200,"data":{"taskId":"ext-456"}}`))
	}))
	defer ts.Close()

	s := &suno{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	result, err := s.Execute(context.Background(), "suno_extend_music", map[string]any{
		"audio_id": "audio-1",
		"prompt":   "Continue with a guitar solo",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "ext-456")
}

func TestGetCredits(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/api/v1/generate/credit", r.URL.Path)
		_, _ = w.Write([]byte(`{"code":200,"msg":"success","data":100}`))
	}))
	defer ts.Close()

	s := &suno{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	result, err := s.Execute(context.Background(), "suno_get_credits", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "100")
}

func TestGenerateLyrics(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/v1/lyrics", r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "A song about adventure", body["prompt"])
		_, _ = w.Write([]byte(`{"code":200,"data":{"taskId":"lyr-789"}}`))
	}))
	defer ts.Close()

	s := &suno{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	result, err := s.Execute(context.Background(), "suno_generate_lyrics", map[string]any{
		"prompt": "A song about adventure",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "lyr-789")
}

func TestGetLyrics(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "lyr-789", r.URL.Query().Get("taskId"))
		_, _ = w.Write([]byte(`{"code":200,"data":{"taskId":"lyr-789","status":"SUCCESS","response":{"text":"Hello world"}}}`))
	}))
	defer ts.Close()

	s := &suno{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	result, err := s.Execute(context.Background(), "suno_get_lyrics", map[string]any{
		"task_id": "lyr-789",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "Hello world")
}

func TestSeparateStems(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/v1/vocal-removal/generate", r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "audio-1", body["audioId"])
		_, _ = w.Write([]byte(`{"code":200,"data":{"taskId":"stem-101"}}`))
	}))
	defer ts.Close()

	s := &suno{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	result, err := s.Execute(context.Background(), "suno_separate_stems", map[string]any{
		"audio_id": "audio-1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "stem-101")
}

func TestConvertWav(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/v1/convert/wav", r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "audio-1", body["audioId"])
		_, _ = w.Write([]byte(`{"code":200,"data":{"taskId":"wav-202"}}`))
	}))
	defer ts.Close()

	s := &suno{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	result, err := s.Execute(context.Background(), "suno_convert_wav", map[string]any{
		"audio_id": "audio-1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "wav-202")
}

func TestCoverAudio(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/v1/generate/cover", r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "https://example.com/song.mp3", body["uploadUrl"])
		_, _ = w.Write([]byte(`{"code":200,"data":{"taskId":"cov-303"}}`))
	}))
	defer ts.Close()

	s := &suno{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	result, err := s.Execute(context.Background(), "suno_cover_audio", map[string]any{
		"upload_url": "https://example.com/song.mp3",
		"style":      "jazz",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "cov-303")
}

func TestGenerateVideo(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/v1/video/generate", r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "audio-1", body["audioId"])
		assert.Equal(t, "Artist Name", body["author"])
		_, _ = w.Write([]byte(`{"code":200,"data":{"taskId":"vid-404"}}`))
	}))
	defer ts.Close()

	s := &suno{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	result, err := s.Execute(context.Background(), "suno_generate_video", map[string]any{
		"audio_id": "audio-1",
		"author":   "Artist Name",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "vid-404")
}

func TestGenerateMashup(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/v1/generate/mashup", r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		ids := body["audioIds"].([]any)
		assert.Len(t, ids, 2)
		assert.Equal(t, "id-1", ids[0])
		assert.Equal(t, "id-2", ids[1])
		_, _ = w.Write([]byte(`{"code":200,"data":{"taskId":"mash-505"}}`))
	}))
	defer ts.Close()

	s := &suno{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	result, err := s.Execute(context.Background(), "suno_generate_mashup", map[string]any{
		"audio_ids": "id-1, id-2",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "mash-505")
}

func TestAddVocals(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/v1/generate/add-vocals", r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "audio-1", body["audioId"])
		_, _ = w.Write([]byte(`{"code":200,"data":{"taskId":"voc-606"}}`))
	}))
	defer ts.Close()

	s := &suno{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	result, err := s.Execute(context.Background(), "suno_add_vocals", map[string]any{
		"audio_id": "audio-1",
		"prompt":   "Soft female vocals",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "voc-606")
}

func TestAddInstrumental(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/v1/generate/add-instrumental", r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "audio-1", body["audioId"])
		_, _ = w.Write([]byte(`{"code":200,"data":{"taskId":"inst-707"}}`))
	}))
	defer ts.Close()

	s := &suno{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	result, err := s.Execute(context.Background(), "suno_add_instrumental", map[string]any{
		"audio_id": "audio-1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "inst-707")
}

func TestGeneratePersona(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/v1/generate/persona", r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		ids := body["audioIds"].([]any)
		assert.Len(t, ids, 2)
		_, _ = w.Write([]byte(`{"code":200,"data":{"taskId":"per-808"}}`))
	}))
	defer ts.Close()

	s := &suno{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	result, err := s.Execute(context.Background(), "suno_generate_persona", map[string]any{
		"audio_ids": "id-1, id-2",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "per-808")
}

func TestGenerateMidi(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/v1/midi/generate", r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "audio-1", body["audioId"])
		_, _ = w.Write([]byte(`{"code":200,"data":{"taskId":"midi-909"}}`))
	}))
	defer ts.Close()

	s := &suno{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	result, err := s.Execute(context.Background(), "suno_generate_midi", map[string]any{
		"audio_id": "audio-1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "midi-909")
}

func TestUploadExtend(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/v1/generate/upload-extend", r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "https://example.com/audio.mp3", body["uploadUrl"])
		_, _ = w.Write([]byte(`{"code":200,"data":{"taskId":"upx-111"}}`))
	}))
	defer ts.Close()

	s := &suno{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	result, err := s.Execute(context.Background(), "suno_upload_extend", map[string]any{
		"upload_url": "https://example.com/audio.mp3",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "upx-111")
}

func TestGetAlignedLyrics(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/v1/lyrics/aligned", r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "audio-1", body["audioId"])
		_, _ = w.Write([]byte(`{"code":200,"data":{"lyrics":[{"text":"Hello","start":0.5,"end":1.2}]}}`))
	}))
	defer ts.Close()

	s := &suno{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	result, err := s.Execute(context.Background(), "suno_get_aligned_lyrics", map[string]any{
		"audio_id": "audio-1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "Hello")
}

func TestGenerateMusic_WithCallbackAndInstrumental(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, true, body["instrumental"])
		assert.Equal(t, "https://example.com/cb", body["callBackUrl"])
		assert.Equal(t, "V5", body["model"])
		_, _ = w.Write([]byte(`{"code":200,"data":{"taskId":"gen-inst"}}`))
	}))
	defer ts.Close()

	s := &suno{apiKey: "key", client: ts.Client(), baseURL: ts.URL}
	result, err := s.Execute(context.Background(), "suno_generate_music", map[string]any{
		"prompt":       "Calm piano instrumental",
		"instrumental": true,
		"callback_url": "https://example.com/cb",
		"model":        "V5",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "gen-inst")
}
