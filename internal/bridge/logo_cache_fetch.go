package bridge

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"goisekai/pkg/types"
)

// fetchLogoBytes downloads a logo URL through the plugin's proxy (respecting
// its pinned TLS profile and headers) and returns the image bytes plus a
// guessed extension.
func (s *AppService) fetchLogoBytes(pluginID, logoURL string) ([]byte, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	reqJSON, _ := json.Marshal(types.HTTPRequest{Method: "GET", URL: logoURL})
	respJSON, err := s.proxy.HandleRequestContext(ctx, pluginID, string(reqJSON))
	if err != nil {
		return nil, "", fmt.Errorf("request: %w", err)
	}
	var resp types.HTTPResponse
	if err := json.Unmarshal([]byte(respJSON), &resp); err != nil {
		return nil, "", fmt.Errorf("unmarshal response: %w", err)
	}
	if resp.Status < 200 || resp.Status >= 300 {
		return nil, "", fmt.Errorf("non-2xx status %d", resp.Status)
	}
	ct := strings.ToLower(resp.Headers["Content-Type"])
	ext := extForMime(ct)
	if ext == "" {
		ext = extFromURL(logoURL)
	}
	return []byte(resp.Body), ext, nil
}

// parseDataURI decodes "data:<mime>;base64,<payload>" into the raw bytes and
// the MIME type. Only base64 payloads are materialised; a missing ";base64"
// marker or a malformed payload yields ok=false.
func parseDataURI(s string) (body []byte, mime string, ok bool) {
	rest, found := strings.CutPrefix(s, "data:")
	if !found {
		return nil, "", false
	}
	before, after, ok := strings.Cut(rest, ",")
	if !ok {
		return nil, "", false
	}
	header, payload := before, after
	parts := strings.Split(header, ";")
	mime = parts[0]
	if len(parts) < 2 || parts[1] != "base64" {
		return nil, "", false
	}
	decoded, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return nil, "", false
	}
	return decoded, mime, true
}

// extForMime maps a content type to a file extension. Empty when unknown.
func extForMime(ct string) string {
	switch strings.ToLower(ct) {
	case "image/png":
		return "png"
	case "image/jpeg":
		return "jpg"
	case "image/jpg":
		return "jpg"
	case "image/webp":
		return "webp"
	case "image/avif":
		return "avif"
	case "image/gif":
		return "gif"
	case "image/svg+xml":
		return "svg"
	}
	return ""
}

// extFromURL guesses the image extension from a URL's path.
func extFromURL(u string) string {
	p, err := url.Parse(u)
	if err != nil {
		return ""
	}
	ext := strings.TrimPrefix(filepath.Ext(p.Path), ".")
	return strings.ToLower(ext)
}

// mimeForBytes sniffs the leading magic bytes of an image and returns a
// content type for extension lookup.
func mimeForBytes(b []byte) string {
	switch {
	case len(b) > 8 && b[0] == 0x89 && b[1] == 'P' && b[2] == 'N' && b[3] == 'G':
		return "image/png"
	case len(b) > 3 && b[0] == 0xFF && b[1] == 0xD8 && b[2] == 0xFF:
		return "image/jpeg"
	case len(b) > 4 && string(b[:4]) == "RIFF":
		return "image/webp"
	case len(b) > 12 && string(b[4:12]) == "ftyp":
		return "image/avif"
	case len(b) > 6 && string(b[:6]) == "GIF87a" || len(b) > 6 && string(b[:6]) == "GIF89a":
		return "image/gif"
	}
	if len(b) > 0 && strings.HasPrefix(string(b[:min(100, len(b))]), "<svg") {
		return "image/svg+xml"
	}
	return ""
}
