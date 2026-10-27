package lsp

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
)

// URIToPath converts an LSP document URI (file://...) to a canonical local filesystem path.
func URIToPath(rawURI string) (string, error) {
	parsed, err := url.Parse(rawURI)
	if err != nil {
		return "", fmt.Errorf("invalid URI %q: %w", rawURI, err)
	}

	if parsed.Scheme != "file" && parsed.Scheme != "" {
		return "", fmt.Errorf("unsupported URI scheme %q", parsed.Scheme)
	}

	unescapedPath, err := url.PathUnescape(parsed.Path)
	if err != nil {
		return "", fmt.Errorf("failed to unescape URI path: %w", err)
	}

	// Handle Windows paths: /C:/dir/file.fdn -> C:/dir/file.fdn
	cleanPath := unescapedPath
	if len(cleanPath) >= 3 && cleanPath[0] == '/' && isWindowsDrive(cleanPath[1:3]) {
		cleanPath = cleanPath[1:]
	}

	return filepath.Clean(filepath.FromSlash(cleanPath)), nil
}

// PathToURI converts a local filesystem path to an RFC 8089 file:// URI.
func PathToURI(path string) string {
	abs, err := filepath.Abs(path)
	if err == nil {
		path = abs
	}
	slashPath := filepath.ToSlash(path)

	// Windows paths starting with "C:" must be prefixed with "/" for URI: /C:/...
	if len(slashPath) >= 2 && isWindowsDrive(slashPath[:2]) {
		slashPath = "/" + slashPath
	} else if !strings.HasPrefix(slashPath, "/") {
		slashPath = "/" + slashPath
	}

	u := url.URL{
		Scheme: "file",
		Path:   slashPath,
	}
	return u.String()
}

func isWindowsDrive(s string) bool {
	if len(s) < 2 {
		return false
	}
	c := s[0]
	isAlpha := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
	return isAlpha && s[1] == ':'
}
