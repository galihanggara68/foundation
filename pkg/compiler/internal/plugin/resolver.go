package plugin

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const executablePermissionMask = 0111

// Resolve locates a generator plugin executable via a prioritized 3-tier lookup:
// 1. Direct path check (./path, ../path, /abs/path).
// 2. Workspace-local binary check (./node_modules/.bin/foundation-gen-<name>, ./bin/foundation-gen-<name>).
// 3. System $PATH lookup (foundation-gen-<name>).
func Resolve(workspaceDir, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("plugin name must not be empty")
	}

	// Tier 1: Direct path check
	if isDirectPath(name) {
		targetPath := name
		if !filepath.IsAbs(targetPath) {
			targetPath = filepath.Join(workspaceDir, targetPath)
		}
		targetPath = filepath.Clean(targetPath)
		if err := verifyExecutable(targetPath); err != nil {
			return "", fmt.Errorf("direct plugin path invalid: %w", err)
		}
		return targetPath, nil
	}

	binName := canonicalBinaryName(name)

	// Tier 2: Workspace-local binary check
	candidates := []string{
		filepath.Join(workspaceDir, "node_modules", ".bin", binName),
		filepath.Join(workspaceDir, "bin", binName),
	}
	for _, candidate := range candidates {
		if err := verifyExecutable(candidate); err == nil {
			return filepath.Clean(candidate), nil
		}
	}

	// Tier 3: System $PATH lookup (foundation-gen-<name> only)
	if absPath, err := lookupInPath(binName); err == nil {
		return absPath, nil
	}

	return "", fmt.Errorf("generator plugin %q could not be resolved (checked direct path, workspace-local ./node_modules/.bin and ./bin, and system $PATH)", name)
}

func isDirectPath(name string) bool {
	return strings.HasPrefix(name, "./") ||
		strings.HasPrefix(name, "../") ||
		strings.HasPrefix(name, "/") ||
		filepath.IsAbs(name)
}

func canonicalBinaryName(name string) string {
	if strings.HasPrefix(name, "foundation-gen-") {
		return name
	}
	return "foundation-gen-" + name
}

func lookupInPath(binName string) (string, error) {
	path, err := exec.LookPath(binName)
	if err != nil {
		return "", err
	}
	if absPath, err := filepath.Abs(path); err == nil {
		return absPath, nil
	}
	return path, nil
}

func verifyExecutable(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("%s is a directory, not an executable file", path)
	}
	if info.Mode()&executablePermissionMask == 0 {
		return fmt.Errorf("%s is not executable (mode: %v)", path, info.Mode())
	}
	return nil
}
