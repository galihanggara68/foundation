package compiler

import (
	"fmt"
	"strings"
)

// RepoMethodKind identifies the operation type of a lowered repository port method.
type RepoMethodKind string

const (
	RepoMethodCreate RepoMethodKind = "create"
	RepoMethodFind   RepoMethodKind = "find"
	RepoMethodUpdate RepoMethodKind = "update"
	RepoMethodDelete RepoMethodKind = "delete"
)

// ArchitectureIR represents the lowered structural topology for a single target.
type ArchitectureIR struct {
	TargetName     string             `json:"target_name"`
	Kind           string             `json:"kind"`
	Language       string             `json:"language"`
	Pattern        string             `json:"pattern"`
	HTTP           string             `json:"http"`
	Module         string             `json:"module"`
	OutDir         string             `json:"out_dir"`
	Layers         []ArchLayer        `json:"layers"`
	DomainEnums    []EnumDecl         `json:"domain_enums,omitempty"`
	DomainValues   []ValueDecl        `json:"domain_values,omitempty"`
	DomainEntities []EntityDecl       `json:"domain_entities,omitempty"`
	DomainErrors   []ErrorDecl        `json:"domain_errors,omitempty"`
	DomainEvents   []EventDecl        `json:"domain_events,omitempty"`
	DomainPolicies []PolicyDecl       `json:"domain_policies,omitempty"`
	Ports          []RepositoryPortIR `json:"ports"`
	UseCases       []UseCaseIR        `json:"use_cases"`
	HTTPHandlers   []HTTPHandlerIR    `json:"http_handlers"`
	Scenarios      []ScenarioDecl     `json:"scenarios,omitempty"`
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
	Name        string      `json:"name"`
	ActionName  string      `json:"action_name"`
	Params      []ParamDecl `json:"params"`
	ReturnType  string      `json:"return_type"`
	Steps       []StepDecl  `json:"steps"`
	RepoDeps    []string    `json:"repo_deps"`
	EmitsEvents bool        `json:"emits_events,omitempty"`
	UsesActor   bool        `json:"uses_actor,omitempty"`
}

type HTTPHandlerIR struct {
	ActionName string      `json:"action_name"`
	Method     string      `json:"method"`
	Path       string      `json:"path"`
	PathParams []string    `json:"path_params,omitempty"`
	Params     []ParamDecl `json:"params,omitempty"`
}

// LowerArchitecture lowers the validated SemanticIR + TargetConfig into an ArchitectureIR
// by dispatching to the registered PatternLowerer strategy for the target.
func LowerArchitecture(sem *SemanticIR, bp *BuildProfile, target TargetConfig) (*ArchitectureIR, []Diagnostic) {
	reg := defaultRegistry()
	key := TargetKey{
		Kind:         target.Kind,
		Language:     target.Language,
		Architecture: target.Architecture,
	}
	lowerer, ok := reg.lowerers[key]
	if !ok {
		return nil, []Diagnostic{{
			Code:     "FDN0300",
			Severity: SeverityError,
			Message:  fmt.Sprintf("unsupported target combination: kind=%s language=%s architecture=%s", target.Kind, target.Language, target.Architecture),
			Span:     Span{File: "foundation.build.yaml", Line: 1, Column: 1},
		}}
	}
	return lowerer.Lower(sem, bp, target)
}

type goHexagonalLowerer struct{}

func (goHexagonalLowerer) Lower(sem *SemanticIR, bp *BuildProfile, target TargetConfig) (*ArchitectureIR, []Diagnostic) {
	arch := &ArchitectureIR{
		TargetName:     target.Name,
		Kind:           target.Kind,
		Language:       target.Language,
		Pattern:        target.Architecture,
		HTTP:           target.HTTP,
		Module:         bp.Project.Module,
		OutDir:         target.OutDir,
		DomainEnums:    sem.Enums,
		DomainValues:   sem.Values,
		DomainEntities: sem.Entities,
		DomainErrors:   sem.Errors,
		DomainEvents:   sem.Events,
		DomainPolicies: sem.Policies,
		Scenarios:      sem.Scenarios,
		Layers: []ArchLayer{
			{Name: "domain", Package: "internal/domain"},
			{Name: "ports", Package: "internal/ports"},
			{Name: "usecases", Package: "internal/usecases"},
			{Name: "adapters_http", Package: "internal/adapters/http"},
			{Name: "adapters_memory", Package: "internal/adapters/memory"},
			{Name: "bootstrap", Package: "internal/bootstrap"},
		},
	}

	entityFields := make(map[string]map[string]string)
	for _, e := range sem.Entities {
		fm := make(map[string]string)
		for _, f := range e.Fields {
			fm[f.Name] = f.Type
		}
		entityFields[e.Name] = fm
	}

	portMap := make(map[string]*RepositoryPortIR)
	var portOrder []string
	for _, e := range sem.Entities {
		p := &RepositoryPortIR{
			InterfaceName: e.Name + "Repository",
			EntityName:    e.Name,
			Methods: []RepositoryMethodIR{
				{
					Kind:       RepoMethodCreate,
					Name:       "Create",
					EntityName: e.Name,
				},
			},
		}
		portMap[e.Name] = p
		portOrder = append(portOrder, e.Name)
	}

	seenPortMethod := make(map[string]bool)
	addPortMethod := func(m RepositoryMethodIR) {
		p := portMap[m.EntityName]
		if p == nil {
			return
		}
		key := m.EntityName + "." + m.Name
		if !seenPortMethod[key] {
			seenPortMethod[key] = true
			p.Methods = append(p.Methods, m)
		}
	}

	for _, a := range sem.Actions {
		depSet := make(map[string]bool)
		var deps []string
		emitsEvents := false
		usesActor := false

		walkStepsAndExprs(a.Steps,
			func(s StepDecl) {
				if s.EntityName != "" && !depSet[s.EntityName] {
					depSet[s.EntityName] = true
					deps = append(deps, s.EntityName)
				}
				if s.Kind == StepEmit {
					emitsEvents = true
				}
				if s.Kind == StepQuery || s.Kind == StepUpdate {
					addPortMethod(RepositoryMethodIR{
						Kind:       RepoMethodFind,
						Name:       "FindBy" + exportIdent(s.WhereField),
						EntityName: s.EntityName,
						WhereField: s.WhereField,
						WhereType:  entityFields[s.EntityName][s.WhereField],
					})
					if s.Kind == StepUpdate {
						addPortMethod(RepositoryMethodIR{
							Kind:       RepoMethodUpdate,
							Name:       "Update",
							EntityName: s.EntityName,
						})
					}
				}
				if s.Kind == StepDelete {
					addPortMethod(RepositoryMethodIR{
						Kind:       RepoMethodDelete,
						Name:       "DeleteBy" + exportIdent(s.WhereField),
						EntityName: s.EntityName,
						WhereField: s.WhereField,
						WhereType:  entityFields[s.EntityName][s.WhereField],
					})
				}
			},
			func(e Expr) {
				if (e.Kind == ExprIdent && e.Value == "actor") || (e.Kind == ExprField && e.Target == "actor") {
					usesActor = true
				}
			},
		)

		arch.UseCases = append(arch.UseCases, UseCaseIR{
			Name:        a.Name + "UseCase",
			ActionName:  a.Name,
			Params:      a.Params,
			ReturnType:  a.ReturnType,
			Steps:       a.Steps,
			RepoDeps:    deps,
			EmitsEvents: emitsEvents,
			UsesActor:   usesActor,
		})

		httpMethod := "POST"
		httpPath := "/" + strings.ToLower(a.Name)
		for _, tr := range a.Traits {
			if tr.Name == "http" {
				if m, ok := tr.Args["method"]; ok {
					httpMethod = strings.ToUpper(m)
				}
				if p, ok := tr.Args["path"]; ok {
					httpPath = p
				}
			}
		}
		var pathParams []string
		for _, seg := range strings.Split(httpPath, "/") {
			if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
				pathParams = append(pathParams, strings.TrimSuffix(strings.TrimPrefix(seg, "{"), "}"))
			}
		}
		arch.HTTPHandlers = append(arch.HTTPHandlers, HTTPHandlerIR{
			ActionName: a.Name,
			Method:     httpMethod,
			Path:       httpPath,
			PathParams: pathParams,
			Params:     a.Params,
		})
	}

	for _, sc := range sem.Scenarios {
		for _, ex := range sc.Expects {
			if ex.StoredEntity != "" {
				addPortMethod(RepositoryMethodIR{
					Kind:       RepoMethodFind,
					Name:       "FindBy" + exportIdent(ex.WhereField),
					EntityName: ex.StoredEntity,
					WhereField: ex.WhereField,
					WhereType:  entityFields[ex.StoredEntity][ex.WhereField],
				})
			}
		}
	}

	for _, name := range portOrder {
		arch.Ports = append(arch.Ports, *portMap[name])
	}

	return arch, nil
}

func exportIdent(s string) string {
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
