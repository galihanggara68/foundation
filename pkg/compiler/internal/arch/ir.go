package arch

import (
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

// PatternLowerer lowers a validated SemanticIR + TargetConfig into an ArchitectureIR (Strategy pattern).
type PatternLowerer interface {
	Lower(sem *ir.SemanticIR, bp *BuildProfile, target TargetConfig) (*ArchitectureIR, []ir.Diagnostic)
}

// RepoMethodKind identifies the operation type of a lowered repository port method.
type RepoMethodKind string

const (
	RepoMethodCreate RepoMethodKind = "create"
	RepoMethodFind   RepoMethodKind = "find"
	RepoMethodUpdate RepoMethodKind = "update"
	RepoMethodDelete RepoMethodKind = "delete"
)

// ArchitectureIR represents the lowered structural topology for a single target.
// It is completely self-contained and never embeds a raw *ir.SemanticIR pointer.
type ArchitectureIR struct {
	TargetName     string             `json:"target_name"`
	Kind           string             `json:"kind"`
	Language       string             `json:"language"`
	Pattern        string             `json:"pattern"`
	HTTP           string             `json:"http"`
	Module         string             `json:"module"`
	OutDir         string             `json:"out_dir"`
	Layers         []ArchLayer        `json:"layers"`
	DomainEnums    []ir.EnumDecl      `json:"domain_enums,omitempty"`
	DomainValues   []ir.ValueDecl     `json:"domain_values,omitempty"`
	DomainEntities []ir.EntityDecl    `json:"domain_entities,omitempty"`
	DomainErrors   []ir.ErrorDecl     `json:"domain_errors,omitempty"`
	DomainEvents   []ir.EventDecl     `json:"domain_events,omitempty"`
	DomainPolicies []ir.PolicyDecl    `json:"domain_policies,omitempty"`
	Ports          []RepositoryPortIR `json:"ports"`
	UseCases       []UseCaseIR        `json:"use_cases"`
	HTTPHandlers   []HTTPHandlerIR    `json:"http_handlers"`
	Scenarios      []ir.ScenarioDecl  `json:"scenarios,omitempty"`
}

type ArchLayer struct {
	Name    string `json:"name"`
	Package string `json:"package"`
}

type RepositoryMethodIR struct {
	Kind       RepoMethodKind `json:"kind"`
	Name       string         `json:"name"`
	EntityName string         `json:"entity_name"`
	WhereField string         `json:"where_field,omitempty"`
	WhereType  string         `json:"where_type,omitempty"`
}

type RepositoryPortIR struct {
	InterfaceName string               `json:"interface_name"`
	EntityName    string               `json:"entity_name"`
	Methods       []RepositoryMethodIR `json:"methods"`
}

type UseCaseIR struct {
	Name        string         `json:"name"`
	ActionName  string         `json:"action_name"`
	Params      []ir.ParamDecl `json:"params"`
	ReturnType  string         `json:"return_type"`
	Steps       []ir.StepDecl  `json:"steps"`
	RepoDeps    []string       `json:"repo_deps"`
	EmitsEvents bool           `json:"emits_events,omitempty"`
	UsesActor   bool           `json:"uses_actor,omitempty"`
}

type HTTPHandlerIR struct {
	ActionName string         `json:"action_name"`
	Method     string         `json:"method"`
	Path       string         `json:"path"`
	PathParams []string       `json:"path_params,omitempty"`
	Params     []ir.ParamDecl `json:"params,omitempty"`
}

// ExportIdent converts a snake_case or camelCase identifier into idiomatic Go PascalCase (preserving ID, SKU, URL acronyms).
func ExportIdent(s string) string {
	if s == "" {
		return ""
	}
	lower := strings.ToLower(s)
	if lower == "id" {
		return "ID"
	}
	if lower == "sku" {
		return "SKU"
	}
	if lower == "url" {
		return "URL"
	}
	parts := strings.Split(s, "_")
	var sb strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		lp := strings.ToLower(p)
		if lp == "id" {
			sb.WriteString("ID")
		} else if lp == "sku" {
			sb.WriteString("SKU")
		} else {
			sb.WriteString(strings.ToUpper(p[:1]) + p[1:])
		}
	}
	return sb.String()
}
