package arch

import (
	"fmt"
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

// PatternLowerer lowers a validated SemanticIR + TargetConfig into an ArchitectureIR (Strategy pattern).
type PatternLowerer interface {
	Lower(sem *ir.SemanticIR, bp *BuildProfile, target TargetConfig) (*ArchitectureIR, []ir.Diagnostic)
}

var defaultLowerers = map[TargetKey]PatternLowerer{
	{Kind: "backend", Language: "go", Architecture: "hexagonal"}:     GoHexagonalLowerer{},
	{Kind: "backend", Language: "go", Architecture: "layered"}:       GoLayeredLowerer{},
	{Kind: "backend", Language: "typescript", Architecture: "clean"}: TSCleanLowerer{},
}

// Lower lowers a validated SemanticIR + TargetConfig into an ArchitectureIR using the matching PatternLowerer strategy.
func Lower(sem *ir.SemanticIR, bp *BuildProfile, target TargetConfig) (*ArchitectureIR, []ir.Diagnostic) {
	key := TargetKey{
		Kind:         target.Kind,
		Language:     target.Language,
		Architecture: target.Architecture,
	}
	lowerer, ok := defaultLowerers[key]
	if !ok {
		return nil, []ir.Diagnostic{{
			Code:     "FDN0300",
			Severity: ir.SeverityError,
			Message:  fmt.Sprintf("unsupported target combination: kind=%s language=%s architecture=%s", target.Kind, target.Language, target.Architecture),
			Span:     ir.Span{File: "foundation.build.yaml", Line: 1, Column: 1},
		}}
	}
	return lowerer.Lower(sem, bp, target)
}

// RepoMethodKind identifies the operation type of a lowered repository port method.
type RepoMethodKind string

const (
	RepoMethodCreate RepoMethodKind = "create"
	RepoMethodFind   RepoMethodKind = "find"
	RepoMethodUpdate RepoMethodKind = "update"
	RepoMethodDelete RepoMethodKind = "delete"
)

type MigrationFileIR struct {
	Number  int    `json:"number"`
	Name    string `json:"name"`
	UpSQL   string `json:"up_sql"`
	DownSQL string `json:"down_sql"`
}

type StoragePlanIR struct {
	Dialect    string            `json:"dialect"`
	Mode       string            `json:"mode"`
	SchemaSQL  string            `json:"schema_sql,omitempty"`
	Migrations []MigrationFileIR `json:"migrations,omitempty"`
}

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
	Storage        StoragePlanIR      `json:"storage"`
	Layers         []ArchLayer        `json:"layers"`
	DomainEnums    []ir.EnumDecl      `json:"domain_enums,omitempty"`
	DomainValues   []ir.ValueDecl     `json:"domain_values,omitempty"`
	DomainEntities []ir.EntityDecl    `json:"domain_entities,omitempty"`
	DomainErrors   []ir.ErrorDecl     `json:"domain_errors,omitempty"`
	DomainEvents   []ir.EventDecl     `json:"domain_events,omitempty"`
	DomainPolicies []ir.PolicyDecl    `json:"domain_policies,omitempty"`
	Slots          []ir.SlotDecl      `json:"slots,omitempty"`
	Ports          []RepositoryPortIR `json:"ports"`
	UseCases       []UseCaseIR        `json:"use_cases"`
	HTTPHandlers   []HTTPHandlerIR    `json:"http_handlers"`
	EventHandlers  []EventHandlerIR   `json:"event_handlers,omitempty"`
	Scenarios      []ir.ScenarioDecl  `json:"scenarios,omitempty"`
}

type EventHandlerIR struct {
	ActionName string `json:"action_name"`
	EventName  string `json:"event_name"`
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
	Name                 string         `json:"name"`
	ActionName           string         `json:"action_name"`
	Params               []ir.ParamDecl `json:"params"`
	ReturnType           string         `json:"return_type"`
	Traits               []ir.TraitDecl `json:"traits,omitempty"`
	Steps                []ir.StepDecl  `json:"steps"`
	RepoDeps             []string       `json:"repo_deps"`
	SlotDeps             []string       `json:"slot_deps,omitempty"`
	EmitsEvents          bool           `json:"emits_events,omitempty"`
	UsesActor            bool           `json:"uses_actor,omitempty"`
	ExpectedVersionParam string         `json:"expected_version_param,omitempty"`
}

// HasTrait looks up a tactical or transport trait on the lowered UseCaseIR by name.
func (uc UseCaseIR) HasTrait(name string) (ir.TraitDecl, bool) {
	for _, tr := range uc.Traits {
		if tr.Name == name {
			return tr, true
		}
	}
	return ir.TraitDecl{}, false
}

// EntityByName looks up a lowered EntityDecl by Entity name (zero value when absent).
func (a *ArchitectureIR) EntityByName(name string) ir.EntityDecl {
	for _, e := range a.DomainEntities {
		if e.Name == name {
			return e
		}
	}
	return ir.EntityDecl{}
}

// EntityPKField returns the primary-key field name (snake_case) and exported Go identifier (PascalCase) for an EntityDecl.
func EntityPKField(ent ir.EntityDecl) (string, string) {
	pkField := "id"
	pkExport := "ID"
	if len(ent.Fields) > 0 {
		pkField = ent.Fields[0].Name
		pkExport = ExportIdent(ent.Fields[0].Name)
	}
	for _, f := range ent.Fields {
		if f.HasTrait("pk") {
			return f.Name, ExportIdent(f.Name)
		}
	}
	return pkField, pkExport
}

// EntityVersionField inspects the entity fields and returns the field name, its exported Go identifier,
// and whether a field with the @version trait exists.
func EntityVersionField(ent ir.EntityDecl) (string, string, bool) {
	for _, f := range ent.Fields {
		if f.HasTrait("version") {
			return f.Name, ExportIdent(f.Name), true
		}
	}
	return "", "", false
}

type HTTPHandlerIR struct {
	ActionName           string         `json:"action_name"`
	Method               string         `json:"method"`
	Path                 string         `json:"path"`
	PathParams           []string       `json:"path_params,omitempty"`
	Params               []ir.ParamDecl `json:"params,omitempty"`
	ExpectedVersionParam string         `json:"expected_version_param,omitempty"`
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

// ToSnakeCase converts PascalCase or camelCase identifiers into deterministic snake_case.
func ToSnakeCase(s string) string {
	var sb strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				sb.WriteByte('_')
			}
			sb.WriteByte(byte(r + ('a' - 'A')))
		} else {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}
