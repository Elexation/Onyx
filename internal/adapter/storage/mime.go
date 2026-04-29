package storage

import (
	"net/http"
	"path/filepath"
	"strings"
)

var extMIME = map[string]string{
	// Text / code
	".md":       "text/markdown",
	".markdown": "text/markdown",
	".yaml":     "text/yaml",
	".yml":      "text/yaml",
	".json":     "application/json",
	".jsonc":    "application/json",
	".toml":     "application/toml",
	".csv":      "text/csv",
	".tsv":      "text/tab-separated-values",
	".log":      "text/plain",
	".ini":      "text/plain",
	".cfg":      "text/plain",
	".conf":     "text/plain",
	".env":      "text/plain",

	// Programming
	".ts":    "text/typescript",
	".tsx":   "text/typescript",
	".jsx":   "text/javascript",
	".mjs":   "text/javascript",
	".cjs":   "text/javascript",
	".vue":   "text/html",
	".svelte": "text/html",
	".go":    "text/x-go",
	".rs":    "text/x-rust",
	".py":    "text/x-python",
	".rb":    "text/x-ruby",
	".sh":    "text/x-shellscript",
	".bat":   "text/x-bat",
	".ps1":   "text/x-powershell",
	".sql":   "text/x-sql",

	// Images
	".svg":  "image/svg+xml",
	".webp": "image/webp",
	".avif": "image/avif",
	".ico":  "image/x-icon",

	// Fonts
	".woff":  "font/woff",
	".woff2": "font/woff2",
	".ttf":   "font/ttf",
	".otf":   "font/otf",

	// Video
	".mp4":  "video/mp4",
	".m4v":  "video/mp4",
	".webm": "video/webm",
	".mov":  "video/quicktime",
	".mkv":  "video/x-matroska",
	".avi":  "video/x-msvideo",
	".ogv":  "video/ogg",

	// Audio
	".mp3":  "audio/mpeg",
	".ogg":  "audio/ogg",
	".oga":  "audio/ogg",
	".wav":  "audio/wav",
	".m4a":  "audio/mp4",
	".aac":  "audio/aac",
	".flac": "audio/flac",
	".opus": "audio/opus",

	// Document
	".pdf": "application/pdf",

	// Binary / application
	".wasm": "application/wasm",
	".gz":   "application/gzip",
	".br":   "application/x-brotli",
	".zst":  "application/zstd",
	".xz":   "application/x-xz",
	".7z":   "application/x-7z-compressed",
}

// DetectMIME returns the MIME type for a file. It checks the extension map
// first, then falls back to http.DetectContentType on the provided header
// bytes (first 512 bytes of the file). Pass nil header to skip magic-byte
// detection and fall back to application/octet-stream.
func DetectMIME(name string, header []byte) string {
	ext := strings.ToLower(filepath.Ext(name))
	if mime, ok := extMIME[ext]; ok {
		return mime
	}
	if len(header) > 0 {
		return http.DetectContentType(header)
	}
	return "application/octet-stream"
}
