package compiler

import (
	"fmt"
	"strings"
)

// ArchitectureIR represents the lowered structural topology for a single target.
type ArchitectureIR struct {
	TargetName  string              `json:"target_name"`
	Kind        string              `json:"kind"`
	Language    string              `json:"language"`
	Pattern     string              `json:"pattern"`
	HTTP        string              `json:"http"`
	Module      string              `json:"module"`
	OutDir      string              `json:"out_dir"`
	Layers      []ArchLayer         `json:"layers"`
	Ports       []RepositoryPortIR  `json:"ports"`
	UseCases    []UseCaseIR         `json:"use_cases"`
	HTTPHandlers []HTTPHandlerIR    `json:"http_handlers"`
	Semantic    *SemanticIR         `json:"semantic"`
}

type ArchLayer struct {
	Name    string `json:"name"`
	Package string `json:"package"`
}

type RepositoryMethodIR struct {
	Name       string `json:"name"`
	EntityName string `json:"entity_name"`
	WhereField string `json:"where_field,omitempty"`
	WhereType  string `json:"where_type,omitempty"`
	IsCreate   bool   `json:"is_create,omitempty"`
}

type RepositoryPortIR struct {
	InterfaceName string               `json:"interface_name"`
	EntityName    string               `json:"entity_name"`
	Methods       []RepositoryMethodIR `json:"methods"`
}

type UseCaseIR struct {
	Name       string     `json:"name"`
	Action     ActionDecl `json:"action"`
	RepoDeps   []string   `json:"repo_deps"`
}

type HTTPHandlerIR struct {
	ActionName string   `json:"action_name"`
	Method     string   `json:"method"`
	Path       string   `json:"path"`
	PathParams []string `json:"path_params,omitempty"`
}

// LowerArchitecture lowers the validated SemanticIR + TargetConfig into an ArchitectureIR.
func LowerArchitecture(sem *SemanticIR, bp *BuildProfile, target TargetConfig) (*ArchitectureIR, []Diagnostic) {
	arch := &ArchitectureIR{
		TargetName: target.Name,
		Kind:       target.Kind,
		Language:   target.Language,
		Pattern:    target.Architecture,
		HTTP:       target.HTTP,
		Module:     bp.Project.Module,
		OutDir:     target.OutDir,
		Semantic:   sem,
	}

	if target.Kind == "backend" && target.Language == "go" && target.Architecture == "hexagonal" {
		arch.Layers = []ArchLayer{
			{Name: "domain", Package: "internal/domain"},
			{Name: "ports", Package: "internal/ports"},
			{Name: "usecases", Package: "internal/usecases"},
			{Name: "adapters_http", Package: "internal/adapters/http"},
			{Name: "adapters_memory", Package: "internal/adapters/memory"},
			{Name: "bootstrap", Package: "internal/bootstrap"},
		}
	} else {
		return nil, []Diagnostic{{
			Code:     "FDN0300",
			Severity: SeverityError,
			Message:  fmt.Sprintf("unsupported target combination: kind=%s language=%s architecture=%s", target.Kind, target.Language, target.Architecture),
			Span:     Span{File: "foundation.build.yaml", Line: 1, Column: 1},
		}}
	}

	entityFields := make(map[string]map[string]string)
	for _, e := range sem.Entities {
		fm := make(map[string]string)
		for _, f := range e.Fields {
			fm[f.Name] = f.Type
		}
		entityFields[e.Name] = fm
	}

	// Build repository ports for each entity
	portMap := make(map[string]*RepositoryPortIR)
	var portOrder []string
	for _, e := range sem.Entities {
		p := &RepositoryPortIR{
			InterfaceName: e.Name + "Repository",
			EntityName:    e.Name,
			Methods: []RepositoryMethodIR{
				{
					Name:       "Create",
					EntityName: e.Name,
					IsCreate:   true,
				},
			},
		}
		portMap[e.Name] = p
		portOrder = append(portOrder, e.Name)
	}

	seenQueryMethod := make(map[string]bool)
	for _, a := range sem.Actions {
		depSet := make(map[string]bool)
		var deps []string
		for _, s := range a.Steps {
			if s.EntityName != "" {
				if !depSet[s.EntityName] {
					depSet[s.EntityName] = true
					deps = append(deps, s.EntityName)
				}
			}
			if s.Kind == StepQuery {
				p := portMap[s.EntityName]
				if p != nil {
					mName := "FindBy" + exportIdent(s.WhereField)
					key := s.EntityName + "." + mName
					if !seenQueryMethod[key] {
						seenQueryMethod[key] = true
						wType := entityFields[s.EntityName][s.WhereField]
						p.Methods = append(p.Methods, RepositoryMethodIR{
							Name:       mName,
							EntityName: s.EntityName,
							WhereField: s.WhereField,
							WhereType:  wType,
						})
					}
				}
			}
		}
		arch.UseCases = append(arch.UseCases, UseCaseIR{
			Name:     a.Name + "UseCase",
			Action:   a,
			RepoDeps: deps,
		})

		// Extract HTTP route from @http trait or convention
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
		})
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
