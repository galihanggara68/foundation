package pluginnew

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/galihanggara68/foundation/pkg/emitterkit"
)

//go:embed templates/*
var templateFS embed.FS

// ScaffoldOptions defines configuration options for plugin generation.
type ScaffoldOptions struct {
	// PluginName defines the name of the plugin.
	PluginName string

	// TargetDir defines the output directory for generated files.
	TargetDir string

	// ModulePath defines the Go module path in go.mod.
	ModulePath string

	// FoundationVersion defines the foundation module version.
	FoundationVersion string

	// ReplacePath defines an optional local filesystem replacement path.
	ReplacePath string
}

// Scaffold generates a new generator plugin project in the target directory.
func Scaffold(opts ScaffoldOptions) error {
	if strings.TrimSpace(opts.PluginName) == "" {
		return errors.New("plugin name is required")
	}
	if opts.TargetDir == "" {
		opts.TargetDir = opts.PluginName
	}
	if opts.ModulePath == "" {
		opts.ModulePath = opts.PluginName
	}
	if opts.FoundationVersion == "" {
		opts.FoundationVersion = "v0.1.0"
	}

	if err := os.MkdirAll(opts.TargetDir, 0755); err != nil {
		return fmt.Errorf("create target directory: %w", err)
	}

	return generateFiles(opts)
}

// generateFiles writes all embedded template files to the target directory.
func generateFiles(opts ScaffoldOptions) error {
	return fs.WalkDir(templateFS, "templates", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}

		relPath := strings.TrimPrefix(path, "templates/")
		targetRelPath := transformTemplatePath(relPath)

		destPath, err := emitterkit.SafeRelPath(opts.TargetDir, targetRelPath)
		if err != nil {
			return fmt.Errorf("validate target path %s: %w", targetRelPath, err)
		}

		data, err := templateFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read embedded template %s: %w", path, err)
		}

		tmpl, err := template.New(relPath).Parse(string(data))
		if err != nil {
			return fmt.Errorf("parse template %s: %w", relPath, err)
		}

		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, opts); err != nil {
			return fmt.Errorf("run template %s: %w", relPath, err)
		}

		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return fmt.Errorf("create parent directory for %s: %w", destPath, err)
		}

		if err := os.WriteFile(destPath, buf.Bytes(), 0644); err != nil {
			return fmt.Errorf("write generated file %s: %w", destPath, err)
		}

		return nil
	})
}

// transformTemplatePath converts a template relative path to a target file path.
func transformTemplatePath(relPath string) string {
	parts := strings.Split(relPath, "/")
	for i, part := range parts {
		if strings.HasPrefix(part, "dot_") {
			parts[i] = "." + strings.TrimPrefix(part, "dot_")
		}
	}
	clean := strings.Join(parts, "/")
	return strings.TrimSuffix(clean, ".tmpl")
}
