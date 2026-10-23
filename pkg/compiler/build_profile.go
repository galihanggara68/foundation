package compiler

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// BuildProfile represents the parsed foundation.build.yaml configuration.
type BuildProfile struct {
	Project ProjectConfig  `json:"project"`
	Storage StorageConfig  `json:"storage"`
	Targets []TargetConfig `json:"targets"`
}

type ProjectConfig struct {
	Name   string `json:"name"`
	Module string `json:"module"`
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
func LoadBuildProfile(workspaceDir string) (*BuildProfile, []Diagnostic) {
	path := filepath.Join(workspaceDir, "foundation.build.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, []Diagnostic{{
			Code:     "FDN0003",
			Severity: SeverityError,
			Message:  fmt.Sprintf("failed to read foundation.build.yaml: %v", err),
			Span:     Span{File: "foundation.build.yaml", Line: 1, Column: 1},
		}}
	}

	bp := &BuildProfile{}
	lines := strings.Split(string(data), "\n")
	section := ""
	var curTarget *TargetConfig

	for i, rawLine := range lines {
		lineNum := i + 1
		// Strip comments
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

	return bp, nil
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

func yamlErr(line int, msg string) []Diagnostic {
	return []Diagnostic{{
		Code:     "FDN0004",
		Severity: SeverityError,
		Message:  msg,
		Span:     Span{File: "foundation.build.yaml", Line: line, Column: 1},
	}}
}
