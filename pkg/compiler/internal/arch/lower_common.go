package arch

import (
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

func lowerBackendCommon(sem *ir.SemanticIR, bp *BuildProfile, target TargetConfig, layers []ArchLayer, useCaseSuffix string) *ArchitectureIR {
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
		Slots:          sem.Slots,
		Scenarios:      sem.Scenarios,
		Layers:         layers,
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
		slotDepSet := make(map[string]bool)
		var slotDeps []string
		emitsEvents := false
		usesActor := false

		ir.WalkStepsAndExprs(a.Steps,
			func(s ir.StepDecl) {
				if s.EntityName != "" && !depSet[s.EntityName] {
					depSet[s.EntityName] = true
					deps = append(deps, s.EntityName)
				}
				if s.Kind == ir.StepSlot && s.SlotName != "" && !slotDepSet[s.SlotName] {
					slotDepSet[s.SlotName] = true
					slotDeps = append(slotDeps, s.SlotName)
				}
				if s.Kind == ir.StepEmit {
					emitsEvents = true
				}
				if s.Kind == ir.StepQueryMany {
					addPortMethod(BuildFindManyRepoMethodIR(
						s.EntityName,
						s.WherePredicates,
						s.OrderByField,
						s.OrderByDesc,
						s.Limit,
						entityFields[s.EntityName],
					))
				}
				if s.Kind == ir.StepQuery || s.Kind == ir.StepUpdate {
					fType := ""
					if entityFields[s.EntityName] != nil {
						fType = entityFields[s.EntityName][s.WhereField]
					}
					addPortMethod(BuildRepoMethodIR(
						RepoMethodFind,
						s.EntityName,
						s.WherePredicates,
						s.WhereField,
						fType,
						entityFields[s.EntityName],
					))
					if s.Kind == ir.StepUpdate {
						addPortMethod(RepositoryMethodIR{
							Kind:       RepoMethodUpdate,
							Name:       "Update",
							EntityName: s.EntityName,
						})
					}
				}
				if s.Kind == ir.StepDelete {
					fType := ""
					if entityFields[s.EntityName] != nil {
						fType = entityFields[s.EntityName][s.WhereField]
					}
					addPortMethod(BuildRepoMethodIR(
						RepoMethodDelete,
						s.EntityName,
						s.WherePredicates,
						s.WhereField,
						fType,
						entityFields[s.EntityName],
					))
				}
			},
			func(e ir.Expr) {
				if (e.Kind == ir.ExprIdent && e.Value == "actor") || (e.Kind == ir.ExprField && e.Target == "actor") {
					usesActor = true
				}
			},
		)

		var expectedVersionParam string
		for _, p := range a.Params {
			if p.Name == "expected_version" || p.Name == "version" {
				expectedVersionParam = p.Name
				break
			}
		}

		isListRet := strings.HasPrefix(a.ReturnType, "[]")
		arch.UseCases = append(arch.UseCases, UseCaseIR{
			Name:                 a.Name + useCaseSuffix,
			ActionName:           a.Name,
			Params:               a.Params,
			ReturnType:           a.ReturnType,
			IsListReturning:      isListRet,
			Traits:               a.Traits,
			Steps:                a.Steps,
			RepoDeps:             deps,
			SlotDeps:             slotDeps,
			EmitsEvents:          emitsEvents,
			UsesActor:            usesActor,
			ExpectedVersionParam: expectedVersionParam,
		})

		for _, tr := range a.Traits {
			if tr.Name == "event_handler" {
				evName, _ := getTraitsArg([]ir.TraitDecl{tr}, "event_handler")
				arch.EventHandlers = append(arch.EventHandlers, EventHandlerIR{
					ActionName: a.Name,
					EventName:  evName,
				})
			}
		}
		if h, ok := LowerActionHTTP(a); ok {
			arch.HTTPHandlers = append(arch.HTTPHandlers, h)
		}
	}

	for _, sc := range sem.Scenarios {
		for _, ex := range sc.Expects {
			if ex.StoredEntity != "" {
				fType := ""
				if entityFields[ex.StoredEntity] != nil {
					fType = entityFields[ex.StoredEntity][ex.WhereField]
				}
				addPortMethod(BuildRepoMethodIR(
					RepoMethodFind,
					ex.StoredEntity,
					ex.WherePredicates,
					ex.WhereField,
					fType,
					entityFields[ex.StoredEntity],
				))
			}
		}
	}

	for _, name := range portOrder {
		arch.Ports = append(arch.Ports, *portMap[name])
	}

	arch.DomainActor = sem.Actor
	arch.AuthnAdapter = bp.Authn

	hasActor := false
	for _, pol := range sem.Policies {
		for _, p := range pol.Params {
			if p.Type == "Actor" || p.Name == "actor" {
				hasActor = true
			}
		}
		ir.WalkExpr(pol.Body, func(e ir.Expr) {
			if (e.Kind == ir.ExprIdent && e.Value == "actor") || (e.Kind == ir.ExprField && e.Target == "actor") {
				hasActor = true
			}
		})
	}
	for _, uc := range arch.UseCases {
		if uc.UsesActor {
			hasActor = true
		}
	}
	arch.HasActorPort = hasActor

	return arch
}

// LowerActionHTTP extracts the HTTPHandlerIR representation for an action if it is exposed.
// Returns false if the action is internal or an event handler.
func LowerActionHTTP(a ir.ActionDecl) (HTTPHandlerIR, bool) {
	isInternal := false
	isEventHandler := false
	httpMethod := "POST"
	isListRet := strings.HasPrefix(a.ReturnType, "[]")
	if isListRet {
		httpMethod = "GET"
	}
	httpPath := "/" + strings.ToLower(a.Name)

	for _, tr := range a.Traits {
		switch tr.Name {
		case "http":
			if m, ok := tr.Args["method"]; ok {
				httpMethod = strings.ToUpper(m)
			}
			if p, ok := tr.Args["path"]; ok {
				httpPath = p
			}
		case "internal":
			isInternal = true
		case "event_handler":
			isEventHandler = true
		}
	}

	if isInternal || isEventHandler {
		return HTTPHandlerIR{}, false
	}

	var pathParams []string
	for _, seg := range strings.Split(httpPath, "/") {
		if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
			pathParams = append(pathParams, strings.TrimSuffix(strings.TrimPrefix(seg, "{"), "}"))
		}
	}

	var expectedVersionParam string
	for _, p := range a.Params {
		if p.Name == "expected_version" || p.Name == "version" {
			expectedVersionParam = p.Name
			break
		}
	}

	return HTTPHandlerIR{
		ActionName:           a.Name,
		Method:               httpMethod,
		Path:                 httpPath,
		PathParams:           pathParams,
		Params:               a.Params,
		ReturnType:           a.ReturnType,
		IsListReturning:      isListRet,
		ExpectedVersionParam: expectedVersionParam,
	}, true
}

// IsScalarType reports whether t is a primitive scalar type in Foundation.
func IsScalarType(t string) bool {
	switch t {
	case "uuid", "string", "int", "decimal", "bool", "timestamp", "float":
		return true
	default:
		return false
	}
}
