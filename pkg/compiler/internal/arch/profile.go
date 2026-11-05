package arch

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/diagnostics"
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
	Authn        string             `json:"authn,omitempty"`
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

// ScaffoldKind defines the scaffolding pre-population mode for generator pipelines.
type ScaffoldKind string

const (
	ScaffoldBuiltin ScaffoldKind = "builtin"
	ScaffoldNone    ScaffoldKind = "none"
)

type TargetConfig struct {
	Name             string            `json:"name"`
	Kind             string            `json:"kind"`
	Language         string            `json:"language"`
	Architecture     string            `json:"architecture"`
	HTTP             string            `json:"http"`
	OutDir           string            `json:"out_dir"`
	Generators       []string          `json:"generators,omitempty"`
	Generator        string            `json:"generator,omitempty"`
	Scaffold         ScaffoldKind      `json:"scaffold,omitempty"`
	Options          map[string]string `json:"options,omitempty"`
	TestCommand      string            `json:"test_command,omitempty"`
	TestTimeout      string            `json:"test_timeout,omitempty"`
	DiagnosticPrefix string            `json:"diagnostic_prefix,omitempty"`
}

// EffectiveGenerators returns the resolved slice of generator plugin names.
func (t TargetConfig) EffectiveGenerators() []string {
	if len(t.Generators) > 0 {
		return t.Generators
	}
	if t.Generator != "" {
		return []string{t.Generator}
	}
	return nil
}

// EffectiveScaffold returns ScaffoldBuiltin or ScaffoldNone, defaulting based on target language and architecture.
func (t TargetConfig) EffectiveScaffold() ScaffoldKind {
	if t.Scaffold != "" {
		return t.Scaffold
	}
	if IsBuiltinTarget(t) {
		return ScaffoldBuiltin
	}
	return ScaffoldNone
}

// LoadBuildProfile reads and parses foundation.build.yaml from workspaceDir.
func LoadBuildProfile(workspaceDir string) (*BuildProfile, []ir.Diagnostic) {
	path := filepath.Join(workspaceDir, "foundation.build.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, []ir.Diagnostic{diagnostics.New("FDN0003", ir.Span{File: "foundation.build.yaml", Line: 1, Column: 1}, err)}
	}

	bp := &BuildProfile{}
	lines := strings.Split(string(data), "\n")
	section := ""
	var curTarget *TargetConfig
	targetSubSection := ""
	targetIndent := 0

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

		if indent == 0 && strings.HasPrefix(trimmed, "authn:") {
			parts := strings.SplitN(trimmed, ":", 2)
			val := strings.TrimSpace(parts[1])
			if val != "" {
				bp.Authn = val
				continue
			}
		}

		if indent == 0 && strings.HasSuffix(trimmed, ":") {
			if curTarget != nil {
				bp.Targets = append(bp.Targets, *curTarget)
				curTarget = nil
			}
			section = strings.TrimSuffix(trimmed, ":")
			continue
		}

		switch section {
		case "authn":
			k, v, ok := splitKV(trimmed)
			if ok && (k == "adapter" || k == "provider") {
				bp.Authn = v
			}
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
			if strings.HasPrefix(trimmed, "-") && (targetSubSection == "" || indent <= targetIndent) {
				if curTarget != nil {
					bp.Targets = append(bp.Targets, *curTarget)
				}
				curTarget = &TargetConfig{}
				targetSubSection = ""
				targetIndent = indent
				rest := strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))
				if rest != "" {
					k, v, ok := splitKV(rest)
					if ok {
						setTargetField(curTarget, k, v)
					}
				}
			} else if curTarget != nil {
				if strings.HasPrefix(trimmed, "-") && targetSubSection == "generators" {
					item := strings.Trim(strings.TrimSpace(strings.TrimPrefix(trimmed, "-")), `"'`)
					if item != "" {
						curTarget.Generators = append(curTarget.Generators, item)
					}
					continue
				}
				if targetSubSection == "options" && indent > targetIndent {
					k, v, ok := splitKV(trimmed)
					if ok {
						if curTarget.Options == nil {
							curTarget.Options = make(map[string]string)
						}
						curTarget.Options[k] = v
						continue
					}
				}
				k, v, ok := splitKV(trimmed)
				if !ok {
					return nil, yamlErr(lineNum, "invalid target property")
				}
				if k == "generators" && v == "" {
					targetSubSection = "generators"
					continue
				}
				if k == "options" && v == "" {
					targetSubSection = "options"
					continue
				}
				targetSubSection = ""
				setTargetField(curTarget, k, v)
			}
		}
	}
	if curTarget != nil {
		bp.Targets = append(bp.Targets, *curTarget)
	}

	for i := range bp.Targets {
		bp.Targets[i].Architecture = NormalizeTargetArchitecture(bp.Targets[i].Kind, bp.Targets[i].Architecture)
		sc := bp.Targets[i].Scaffold
		if sc != "" && sc != ScaffoldBuiltin && sc != ScaffoldNone {
			return nil, []ir.Diagnostic{diagnostics.NewWithDetail(
				"FDN0004",
				ir.Span{File: "foundation.build.yaml", Line: 1, Column: 1},
				fmt.Sprintf("invalid scaffold mode %q in target %q; expected 'builtin' or 'none'", sc, bp.Targets[i].Name),
			)}
		}
		if tt := bp.Targets[i].TestTimeout; tt != "" {
			d, err := time.ParseDuration(tt)
			if err != nil || d <= 0 {
				return nil, []ir.Diagnostic{diagnostics.NewWithDetail(
					"FDN0004",
					ir.Span{File: "foundation.build.yaml", Line: 1, Column: 1},
					fmt.Sprintf("invalid test_timeout %q in target %q: must be a positive duration", tt, bp.Targets[i].Name),
				)}
			}
		}
	}

	if prefixDiags := validateDiagnosticPrefixes(bp.Targets); len(prefixDiags) > 0 {
		return nil, prefixDiags
	}

	if bp.Project.Module == "" {
		bp.Project.Module = "example.com/" + bp.Project.Name
	}

	if bp.Authn == "" {
		bp.Authn = "dev-headers"
	} else if bp.Authn != "dev-headers" && bp.Authn != "jwt-oidc" {
		return nil, []ir.Diagnostic{diagnostics.New(
			"FDN0310",
			ir.Span{File: "foundation.build.yaml", Line: 1, Column: 1},
			bp.Authn,
		)}
	}

	if bp.Dependencies.Runtime != "" {
		if !isSemVerCompatible(ContribRuntimeVersion, bp.Dependencies.Runtime) {
			return nil, []ir.Diagnostic{diagnostics.New(
				"FDN0007",
				ir.Span{File: "foundation.build.yaml", Line: 1, Column: 1},
				bp.Dependencies.Runtime,
				ContribRuntimeVersion,
			)}
		}
	}

	return bp, nil
}

// ContribRuntimeVersion is the contrib/go/runtime release the compiler emits into a generated
// go.mod and accepts through the build profile's dependencies.runtime. The emitted pin, its
// checksum and the compatibility check all read it from here, so a runtime release cannot
// leave them disagreeing.
const ContribRuntimeVersion = "0.2.0"

func isSemVerCompatible(requiredVer, pinned string) bool {
	clean := strings.TrimLeft(pinned, "^~>=v ")
	parts := strings.Split(clean, ".")
	cClean := strings.TrimLeft(requiredVer, "v ")
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

func parseFlowList(v string) []string {
	v = strings.TrimSpace(v)
	if strings.HasPrefix(v, "[") && strings.HasSuffix(v, "]") {
		v = v[1 : len(v)-1]
	}
	parts := strings.Split(v, ",")
	var out []string
	for _, p := range parts {
		item := strings.TrimSpace(p)
		item = strings.Trim(item, `"'`)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
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
	case "generator":
		t.Generator = v
	case "generators":
		if strings.HasPrefix(v, "[") {
			t.Generators = parseFlowList(v)
		} else if v != "" {
			t.Generators = []string{v}
		}
	case "scaffold":
		t.Scaffold = ScaffoldKind(v)
	case "test_command":
		t.TestCommand = v
	case "test_timeout":
		t.TestTimeout = v
	case "diagnostic_prefix":
		t.DiagnosticPrefix = v
	}
}

// BuildProfileSpan returns the diagnostic span the compiler uses for foundation.build.yaml errors.
func BuildProfileSpan() ir.Span {
	return ir.Span{File: "foundation.build.yaml", Line: 1, Column: 1}
}

// reservedDiagnosticPrefix is the diagnostic prefix the compiler reserves for its own codes.
const reservedDiagnosticPrefix = "FDN"

// validateDiagnosticPrefixes reserves the FDN prefix and rejects duplicate non-empty
// diagnostic prefixes across targets, so two plugins cannot claim the same codes.
func validateDiagnosticPrefixes(targets []TargetConfig) []ir.Diagnostic {
	var diags []ir.Diagnostic
	span := BuildProfileSpan()

	owners := make(map[string][]string)
	for _, t := range targets {
		if t.DiagnosticPrefix == "" {
			continue
		}
		if t.DiagnosticPrefix == reservedDiagnosticPrefix {
			diags = append(diags, diagnostics.New("FDN0604", span, t.DiagnosticPrefix))
			continue
		}
		owners[t.DiagnosticPrefix] = append(owners[t.DiagnosticPrefix], t.Name)
	}

	prefixes := make([]string, 0, len(owners))
	for prefix := range owners {
		prefixes = append(prefixes, prefix)
	}
	sort.Strings(prefixes)
	for _, prefix := range prefixes {
		names := owners[prefix]
		if len(names) < 2 {
			continue
		}
		sort.Strings(names)
		diags = append(diags, diagnostics.New("FDN0603", span, prefix, strings.Join(names, ", ")))
	}
	return diags
}

func yamlErr(line int, msg string) []ir.Diagnostic {
	return []ir.Diagnostic{diagnostics.NewWithDetail(
		"FDN0004",
		ir.Span{File: "foundation.build.yaml", Line: line, Column: 1},
		msg,
	)}
}
