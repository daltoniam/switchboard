package botidentity

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"image"
	"image/jpeg"
	"image/png"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	mcp "github.com/daltoniam/switchboard"
)

const (
	defaultImageModelID = "stability.stable-image-core-v1:1"
	maxLogoBytes        = 900 * 1024
)

func generateLogo(ctx context.Context, b *botidentity, args map[string]any) (*mcp.ToolResult, error) {
	if b.bedrockClient == nil {
		return mcp.ErrResult(fmt.Errorf("AWS credentials not configured — set aws_access_key_id and aws_secret_access_key, or configure default AWS credentials"))
	}

	initialPrompt, _ := mcp.ArgStr(args, "prompt")
	initialNeg, _ := mcp.ArgStr(args, "negative_prompt")
	modelID, _ := mcp.ArgStr(args, "model_id")
	if modelID == "" {
		modelID = defaultImageModelID
	}
	botID, _ := mcp.ArgStr(args, "bot_id")

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return mcp.ErrResult(fmt.Errorf("failed to start listener: %w", err))
	}
	port := listener.Addr().(*net.TCPAddr).Port

	type result struct {
		path   string
		format string
		size   int
	}
	resultCh := make(chan result, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		data := struct {
			Prompt   string
			Negative string
		}{Prompt: initialPrompt, Negative: initialNeg}
		if err := logoPreviewPage.Execute(w, data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	mux.HandleFunc("/generate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}
		prompt := r.FormValue("prompt")
		neg := r.FormValue("negative_prompt")
		if prompt == "" {
			http.Error(w, "prompt required", http.StatusBadRequest)
			return
		}

		imgB64, err := callBedrock(ctx, b.bedrockClient, modelID, prompt, neg)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		raw, _ := base64.StdEncoding.DecodeString(imgB64)
		compressed, _, _ := compressForLogo(raw)
		b64out := base64.StdEncoding.EncodeToString(compressed)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"image": b64out})
	})

	mux.HandleFunc("/confirm", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}
		imgB64 := r.FormValue("image")
		if imgB64 == "" {
			http.Error(w, "no image", http.StatusBadRequest)
			return
		}

		imgBytes, _ := base64.StdEncoding.DecodeString(imgB64)

		ext := ".jpg"
		format := "jpeg"
		if len(imgBytes) > 4 && imgBytes[0] == 0x89 && imgBytes[1] == 0x50 {
			ext = ".png"
			format = "png"
		}

		var path string
		if botID != "" && b.inv != nil {
			p, err := b.inv.saveLogo(botID, imgBytes, ext)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			path = p
			_ = b.inv.update(botID, func(e *BotEntry) {
				e.LogoPath = path
			})
		} else {
			p, err := writeTempFile(imgBytes, "botlogo-*"+ext)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			path = p
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprintf(w, `<html><body><h2>Logo saved!</h2><p>%s</p><p>You can close this tab.</p></body></html>`, html.EscapeString(path))
		resultCh <- result{path: path, format: format, size: len(imgBytes)}
	})

	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		if err := srv.Serve(listener); err != nil && !strings.Contains(err.Error(), "Server closed") {
			errCh <- err
		}
	}()

	localURL := fmt.Sprintf("http://localhost:%d", port)
	_ = openBrowser(localURL)

	select {
	case res := <-resultCh:
		shutCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)

		return mcp.JSONResult(map[string]any{
			"path":     res.path,
			"format":   res.format,
			"size":     res.size,
			"model_id": modelID,
		})

	case err := <-errCh:
		return mcp.ErrResult(fmt.Errorf("local server error: %w", err))

	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
		return mcp.ErrResult(fmt.Errorf("timed out — a browser window should have opened at %s", localURL))
	}
}

func callBedrock(ctx context.Context, client *bedrockruntime.Client, modelID, prompt, negPrompt string) (string, error) {
	var reqJSON []byte
	var err error

	if strings.HasPrefix(modelID, "stability.") {
		req := map[string]any{
			"prompt": prompt, "mode": "text-to-image", "output_format": "png", "aspect_ratio": "1:1",
		}
		if negPrompt != "" {
			req["negative_prompt"] = negPrompt
		}
		reqJSON, err = json.Marshal(req)
	} else {
		req := map[string]any{
			"taskType":              "TEXT_IMAGE",
			"textToImageParams":     map[string]any{"text": prompt},
			"imageGenerationConfig": map[string]any{"numberOfImages": 1, "width": 512, "height": 512, "quality": "standard", "cfgScale": 8.0, "seed": 0},
		}
		if negPrompt != "" {
			req["textToImageParams"].(map[string]any)["negativeText"] = negPrompt
		}
		reqJSON, err = json.Marshal(req)
	}
	if err != nil {
		return "", err
	}

	resp, err := client.InvokeModel(ctx, &bedrockruntime.InvokeModelInput{
		ModelId: strPtr(modelID), ContentType: strPtr("application/json"), Accept: strPtr("application/json"), Body: reqJSON,
	})
	if err != nil {
		return "", fmt.Errorf("bedrock: %w", err)
	}

	var result struct {
		Images []string `json:"images"`
		Error  string   `json:"error"`
	}
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return "", err
	}
	if result.Error != "" {
		return "", fmt.Errorf("%s", result.Error)
	}
	if len(result.Images) == 0 {
		return "", fmt.Errorf("no images returned")
	}
	return result.Images[0], nil
}

func compressForLogo(pngData []byte) ([]byte, string, error) {
	if len(pngData) <= maxLogoBytes {
		return pngData, "png", nil
	}

	img, err := png.Decode(bytes.NewReader(pngData))
	if err != nil {
		return nil, "", fmt.Errorf("decode png: %w", err)
	}

	for _, q := range []int{90, 80, 70, 60} {
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: q}); err != nil {
			return nil, "", fmt.Errorf("encode jpeg: %w", err)
		}
		if buf.Len() <= maxLogoBytes {
			return buf.Bytes(), "jpeg", nil
		}
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 50}); err != nil {
		return nil, "", fmt.Errorf("encode jpeg: %w", err)
	}
	return buf.Bytes(), "jpeg", nil
}

var _ image.Config

func writeTempFile(data []byte, pattern string) (string, error) {
	f, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return "", err
	}
	_ = f.Close()
	return f.Name(), nil
}

func strPtr(s string) *string { return &s }

var logoPreviewPage = template.Must(template.New("logo").Parse(`<!DOCTYPE html>
<html>
<head>
<title>Bot Logo Generator</title>
<style>
  body { font-family: -apple-system, sans-serif; max-width: 700px; margin: 40px auto; padding: 0 20px; background: #f5f5f5; }
  h1 { color: #333; }
  textarea { width: 100%; height: 80px; font-size: 14px; padding: 8px; border: 1px solid #ccc; border-radius: 6px; resize: vertical; }
  label { font-weight: 600; display: block; margin: 12px 0 4px; }
  button { padding: 10px 24px; font-size: 15px; border: none; border-radius: 6px; cursor: pointer; margin: 8px 8px 8px 0; }
  .gen { background: #2563eb; color: white; }
  .gen:hover { background: #1d4ed8; }
  .gen:disabled { background: #93c5fd; cursor: wait; }
  .confirm { background: #16a34a; color: white; }
  .confirm:hover { background: #15803d; }
  .confirm:disabled { background: #86efac; }
  #preview { margin: 20px 0; text-align: center; }
  #preview img { max-width: 512px; border-radius: 12px; box-shadow: 0 2px 12px rgba(0,0,0,0.15); }
  .status { color: #666; font-style: italic; min-height: 24px; }
  .error { color: #dc2626; }
</style>
</head>
<body>
<h1>Bot Logo Generator</h1>
<form id="form" onsubmit="return generate()">
  <label for="prompt">Prompt</label>
  <textarea id="prompt" name="prompt">{{.Prompt}}</textarea>
  <label for="negative_prompt">Negative prompt</label>
  <textarea id="negative_prompt" name="negative_prompt" style="height:40px">{{.Negative}}</textarea>
  <div>
    <button type="submit" class="gen" id="genBtn">Generate</button>
    <button type="button" class="confirm" id="confirmBtn" disabled onclick="confirm()">Use this logo</button>
  </div>
</form>
<div class="status" id="status"></div>
<div id="preview"></div>

<script>
var currentImage = null;
function setStatus(msg, isError) {
  var el = document.getElementById('status');
  el.textContent = msg;
  el.className = isError ? 'status error' : 'status';
}
function generate() {
  var btn = document.getElementById('genBtn');
  btn.disabled = true;
  setStatus('Generating... (this takes ~10 seconds)', false);
  document.getElementById('confirmBtn').disabled = true;
  var fd = new FormData(document.getElementById('form'));
  var xhr = new XMLHttpRequest();
  xhr.open('POST', '/generate');
  xhr.onload = function() {
    btn.disabled = false;
    try {
      var r = JSON.parse(xhr.responseText);
      if (r.error) { setStatus('Error: ' + r.error, true); return; }
      currentImage = r.image;
      document.getElementById('preview').innerHTML = '<img src="data:image/jpeg;base64,' + r.image + '">';
      document.getElementById('confirmBtn').disabled = false;
      setStatus('Generated! Click "Use this logo" to save, or edit the prompt and regenerate.', false);
    } catch(e) { setStatus('Error: ' + e, true); }
  };
  xhr.onerror = function() { btn.disabled = false; setStatus('Network error', true); };
  xhr.send(fd);
  return false;
}
function confirm() {
  if (!currentImage) return;
  var fd = new FormData();
  fd.append('image', currentImage);
  var xhr = new XMLHttpRequest();
  xhr.open('POST', '/confirm');
  xhr.onload = function() { document.body.innerHTML = xhr.responseText; };
  xhr.send(fd);
}
</script>
</body>
</html>`))
