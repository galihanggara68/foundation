package arch

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

// TargetKey uniquely identifies a compilation target strategy by kind, language, and architecture.
type TargetKey struct {
	Kind         string
	Language     string
	Architecture string
}

// BuildProfile represents the parsed foundation.build.yaml configuration.
type BuildProfile struct {
	Project      ProjectConfig      `json:"project"`
	Storage      StorageConfig      `json:"storage"`
	Dependencies DependenciesConfig `json:"dependencies"`
	Targets      []TargetConfig     `json:"targets"`
}

type ProjectConfig struct {
	Name   string `json:"name"`
	Module string `json:"module"`
}

type DependenciesConfig struct {
	Runtime string `json:"runtime"`
}

type StorageConfig struct {
	Dialect string `json:"dialect"`
	Mode    string `json:"mode"`
}

type TargetConfig struct {
	Name         string `json:"name"`
	Kind         string `json:"kind"`
	Language     string `json:"language"`
	Architecture string `json:"architecture"`
	HTTP         string `json:"http"`
	OutDir       string `json:"out_dir"`
}

// LoadBuildProfile reads and parses foundation.build.yaml from workspaceDir.
func LoadBuildProfile(workspaceDir string) (*BuildProfile, []ir.Diagnostic) {
	path := filepath.Join(workspaceDir, "foundation.build.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, []ir.Diagnostic{{
			Code:     "FDN0003",
			Severity: ir.SeverityError,
			Message:  fmt.Sprintf("failed to read foundation.build.yaml: %v", err),
			Span:     ir.Span{File: "foundation.build.yaml", Line: 1, Column: 1},
		}}
	}

	bp := &BuildProfile{}
	lines := strings.Split(string(data), "\n")
	section := ""
	var curTarget *TargetConfig

	for i, rawLine := range lines {
		lineNum := i + 1
		if idx := strings.Index(rawLine, "#"); idx >= 0 {
			rawLine = rawLine[:idx]
		}
		trimmed := strings.TrimSpace(rawLine)
		if trimmed == "" {
			continue
		}
		indent := len(rawLine) - len(strings.TrimLeft(rawLine, " \t"))

		if indent == 0 && strings.HasSuffix(trimmed, ":") {
			if curTarget != nil {
				bp.Targets = append(bp.Targets, *curTarget)
				curTarget = nil
			}
			section = strings.TrimSuffix(trimmed, ":")
			continue
		}

		switch section {
		case "project":
			k, v, ok := splitKV(trimmed)
			if !ok {
				return nil, yamlErr(lineNum, "invalid project key-value")
			}
			switch k {
			case "name":
				bp.Project.Name = v
			case "module":
				bp.Project.Module = v
			}
		case "storage":
			k, v, ok := splitKV(trimmed)
			if !ok {
				return nil, yamlErr(lineNum, "invalid storage key-value")
			}
			switch k {
			case "dialect":
				bp.Storage.Dialect = v
			case "mode":
				bp.Storage.Mode = v
			}
		case "dependencies":
			k, v, ok := splitKV(trimmed)
			if !ok {
				return nil, yamlErr(lineNum, "invalid dependencies key-value")
			}
			if k == "runtime" {
				bp.Dependencies.Runtime = v
			}
		case "targets":
			if strings.HasPrefix(trimmed, "-") {
				if curTarget != nil {
					bp.Targets = append(bp.Targets, *curTarget)
				}
				curTarget = &TargetConfig{}
				rest := strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))
				if rest != "" {
					k, v, ok := splitKV(rest)
					if ok {
						setTargetField(curTarget, k, v)
					}
				}
			} else if curTarget != nil {
				k, v, ok := splitKV(trimmed)
				if !ok {
					return nil, yamlErr(lineNum, "invalid target property")
				}
				setTargetField(curTarget, k, v)
			}
		}
	}
	if curTarget != nil {
		bp.Targets = append(bp.Targets, *curTarget)
	}

	if bp.Project.Module == "" {
		bp.Project.Module = "example.com/" + bp.Project.Name
	}

	if bp.Dependencies.Runtime != "" {
		if !isSemVerCompatible("0.1.0", bp.Dependencies.Runtime) {
			return nil, []ir.Diagnostic{{
				Code:     "FDN0007",
				Severity: ir.SeverityError,
				Message:  fmt.Sprintf("incompatible contrib runtime version %q: compiler v0.1.0 requires ~0.1.0", bp.Dependencies.Runtime),
				Span:     ir.Span{File: "foundation.build.yaml", Line: 1, Column: 1},
			}}
		}
	}

	return bp, nil
}

func isSemVerCompatible(compilerVer, pinned string) bool {
	clean := strings.TrimLeft(pinned, "^~>=v ")
	parts := strings.Split(clean, ".")
	cClean := strings.TrimLeft(compilerVer, "v ")
	cParts := strings.Split(cClean, ".")
	if len(parts) < 2 || len(cParts) < 2 {
		return false
	}
	// For 0.y.z versions, minor version must match exactly (SemVer 0.x rule)
	if cParts[0] == "0" {
		return parts[0] == "0" && parts[1] == cParts[1]
	}
	// For major >= 1, major version must match
	return parts[0] == cParts[0]
}

func splitKV(s string) (string, string, bool) {
	idx := strings.Index(s, ":")
	if idx < 0 {
		return "", "", false
	}
	k := strings.TrimSpace(s[:idx])
	v := strings.TrimSpace(s[idx+1:])
	v = strings.Trim(v, `"'`)
	return k, v, true
}

func setTargetField(t *TargetConfig, k, v string) {
	switch k {
	case "name":
		t.Name = v
	case "kind":
		t.Kind = v
	case "language":
		t.Language = v
	case "architecture":
		t.Architecture = v
	case "http":
		t.HTTP = v
	case "out_dir":
		t.OutDir = v
	}
}

func yamlErr(line int, msg string) []ir.Diagnostic {
	return []ir.Diagnostic{{
		Code:     "FDN0004",
		Severity: ir.SeverityError,
		Message:  msg,
		Span:     ir.Span{File: "foundation.build.yaml", Line: line, Column: 1},
	}}
}
