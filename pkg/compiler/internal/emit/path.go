package emit

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"
)

// SafeRelPath confirms that rel is a clean relative path that stays within rootDir.
// The function rejects empty paths, root paths, absolute paths, and directory traversal sequences.
func SafeRelPath(rootDir, rel string) (string, error) {
	norm := strings.ReplaceAll(rel, "\\", "/")
	if strings.TrimSpace(norm) == "" || norm == "." {
		return "", fmt.Errorf("invalid empty or root file path: %q", rel)
	}
	if len(norm) >= 2 && norm[1] == ':' && isAlpha(norm[0]) {
		return "", fmt.Errorf("security violation: absolute file path %q is forbidden", rel)
	}
	if strings.HasPrefix(norm, "/") || filepath.IsAbs(rel) {
		return "", fmt.Errorf("security violation: absolute file path %q is forbidden", rel)
	}

	cleanRel := path.Clean(norm)
	if cleanRel == "." {
		return "", fmt.Errorf("invalid empty or root file path: %q", rel)
	}
	if cleanRel == ".." || strings.HasPrefix(cleanRel, "../") {
		return "", fmt.Errorf("security violation: file path %q escapes root directory", rel)
	}

	diskPath := filepath.Join(rootDir, filepath.FromSlash(cleanRel))
	relToRoot, err := filepath.Rel(rootDir, diskPath)
	if err != nil || relToRoot == ".." || strings.HasPrefix(relToRoot, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("security violation: resolved path %q escapes root directory %q", diskPath, rootDir)
	}

	return diskPath, nil
}

func isAlpha(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}
