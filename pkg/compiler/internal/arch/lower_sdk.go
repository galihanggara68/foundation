package arch

import (
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

// GoSDKLowerer lowers a SemanticIR and TargetConfig into an ArchitectureIR for Go client SDK emission.
type GoSDKLowerer struct{}

func (GoSDKLowerer) Lower(sem *ir.SemanticIR, bp *BuildProfile, target TargetConfig) (*ArchitectureIR, []ir.Diagnostic) {
	return lowerSDKCommon(sem, bp, target)
}

// TSSDKLowerer lowers a SemanticIR and TargetConfig into an ArchitectureIR for TypeScript client SDK emission.
type TSSDKLowerer struct{}

func (TSSDKLowerer) Lower(sem *ir.SemanticIR, bp *BuildProfile, target TargetConfig) (*ArchitectureIR, []ir.Diagnostic) {
	return lowerSDKCommon(sem, bp, target)
}

func lowerSDKCommon(sem *ir.SemanticIR, bp *BuildProfile, target TargetConfig) (*ArchitectureIR, []ir.Diagnostic) {
	arch := &ArchitectureIR{
		TargetName:     target.Name,
		Kind:           target.Kind,
		Language:       target.Language,
		Pattern:        target.Architecture,
		Module:         bp.Project.Module,
		OutDir:         target.OutDir,
		DomainEnums:    sem.Enums,
		DomainValues:   sem.Values,
		DomainEntities: sem.Entities,
		DomainErrors:   sem.Errors,
	}

	for _, a := range sem.Actions {
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
			case "internal":
				isInternal = true
			case "event_handler":
				isEventHandler = true
			case "http":
				if m, ok := tr.Args["method"]; ok {
					httpMethod = strings.ToUpper(m)
				}
				if p, ok := tr.Args["path"]; ok {
					httpPath = p
				}
			}
		}

		if isInternal || isEventHandler {
			continue
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

		arch.HTTPHandlers = append(arch.HTTPHandlers, HTTPHandlerIR{
			ActionName:           a.Name,
			Method:               httpMethod,
			Path:                 httpPath,
			PathParams:           pathParams,
			Params:               a.Params,
			ReturnType:           a.ReturnType,
			IsListReturning:      isListRet,
			ExpectedVersionParam: expectedVersionParam,
		})

		arch.UseCases = append(arch.UseCases, UseCaseIR{
			Name:                 a.Name,
			ActionName:           a.Name,
			Params:               a.Params,
			ReturnType:           a.ReturnType,
			IsListReturning:      isListRet,
			Traits:               a.Traits,
			Steps:                a.Steps,
			ExpectedVersionParam: expectedVersionParam,
		})
	}

	return arch, nil
}
