package compiler

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Span represents an exact source location in a Blueprint file.
type Span struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
}

func (s Span) String() string {
	return fmt.Sprintf("%s:%d:%d", s.File, s.Line, s.Column)
}

// Severity indicates whether a diagnostic is an error or warning.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

// Diagnostic reports a syntax, type, or architectural invariant violation.
type Diagnostic struct {
	Code     string   `json:"code"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
	Span     Span     `json:"span"`
}

// SemanticIR represents the validated, architecture-agnostic domain representation.
type SemanticIR struct {
	Enums     []EnumDecl     `json:"enums,omitempty"`
	Values    []ValueDecl    `json:"values,omitempty"`
	Entities  []EntityDecl   `json:"entities,omitempty"`
	Errors    []ErrorDecl    `json:"errors,omitempty"`
	Events    []EventDecl    `json:"events,omitempty"`
	Policies  []PolicyDecl   `json:"policies,omitempty"`
	Actions   []ActionDecl   `json:"actions,omitempty"`
	Scenarios []ScenarioDecl `json:"scenarios,omitempty"`
}

type EnumDecl struct {
	Name     string   `json:"name"`
	Variants []string `json:"variants"`
	Span     Span     `json:"span"`
}

type EventDecl struct {
	Name   string      `json:"name"`
	Fields []FieldDecl `json:"fields"`
	Span   Span        `json:"span"`
}

type PolicyDecl struct {
	Name   string      `json:"name"`
	Params []ParamDecl `json:"params"`
	Body   Expr        `json:"body"`
	Span   Span        `json:"span"`
}

type FieldDecl struct {
	Name       string      `json:"name"`
	Type       string      `json:"type"`
	Optional   bool        `json:"optional,omitempty"`
	DefaultVal string      `json:"default_val,omitempty"`
	Traits     []TraitDecl `json:"traits,omitempty"`
	Span       Span        `json:"span"`
}

type TraitDecl struct {
	Name string            `json:"name"`
	Args map[string]string `json:"args,omitempty"`
	Span Span              `json:"span"`
}

type ValueDecl struct {
	Name   string      `json:"name"`
	Fields []FieldDecl `json:"fields"`
	Span   Span        `json:"span"`
}

type EntityDecl struct {
	Name   string      `json:"name"`
	Fields []FieldDecl `json:"fields"`
	Span   Span        `json:"span"`
}

type ErrorDecl struct {
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	Message string `json:"message"`
	Span    Span   `json:"span"`
}

type ParamDecl struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Span Span   `json:"span"`
}

type ActionDecl struct {
	Name       string      `json:"name"`
	Params     []ParamDecl `json:"params"`
	ReturnType string      `json:"return_type"`
	Traits     []TraitDecl `json:"traits,omitempty"`
	Steps      []StepDecl  `json:"steps"`
	Span       Span        `json:"span"`
}

type StepKind string

const (
	StepLet     StepKind = "let"
	StepQuery   StepKind = "query"
	StepCreate  StepKind = "create"
	StepUpdate  StepKind = "update"
	StepDelete  StepKind = "delete"
	StepRequire StepKind = "require"
	StepGuard   StepKind = "guard"
	StepEmit    StepKind = "emit"
	StepIf      StepKind = "if"
	StepMatch   StepKind = "match"
	StepFor     StepKind = "for"
	StepWhile   StepKind = "while"
	StepReturn  StepKind = "return"
)

type ExprKind string

const (
	ExprIdent      ExprKind = "ident"
	ExprField      ExprKind = "field"
	ExprString     ExprKind = "string"
	ExprInt        ExprKind = "int"
	ExprFloat      ExprKind = "float"
	ExprBool       ExprKind = "bool"
	ExprEnumLit    ExprKind = "enum_lit"
	ExprBinary     ExprKind = "binary"
	ExprConstruct  ExprKind = "construct"
	ExprList       ExprKind = "list"
	ExprCombinator ExprKind = "combinator"
	ExprCall       ExprKind = "call"
)

type Expr struct {
	Kind         ExprKind        `json:"kind"`
	Value        string          `json:"value,omitempty"`
	Target       string          `json:"target,omitempty"`
	Field        string          `json:"field,omitempty"`
	Op           string          `json:"op,omitempty"`
	Left         *Expr           `json:"left,omitempty"`
	Right        *Expr           `json:"right,omitempty"`
	TypeName     string          `json:"type_name,omitempty"`
	Fields       []FieldInitDecl `json:"fields,omitempty"`
	Items        []Expr          `json:"items,omitempty"`
	InferredType string          `json:"inferred_type,omitempty"`
	Span         Span            `json:"span"`
}

type FieldInitDecl struct {
	Name  string `json:"name"`
	Value Expr   `json:"value"`
	Span  Span   `json:"span"`
}

type MatchArmDecl struct {
	Variant string     `json:"variant"`
	Steps   []StepDecl `json:"steps"`
	Span    Span       `json:"span"`
}

type StepDecl struct {
	Kind       StepKind        `json:"kind"`
	VarName    string          `json:"var_name,omitempty"`
	EntityName string          `json:"entity_name,omitempty"`
	PolicyName string          `json:"policy_name,omitempty"`
	PolicyArgs []Expr          `json:"policy_args,omitempty"`
	EventName  string          `json:"event_name,omitempty"`
	WhereField string          `json:"where_field,omitempty"`
	WhereValue *Expr           `json:"where_value,omitempty"`
	ElseError  string          `json:"else_error,omitempty"`
	Fields     []FieldInitDecl `json:"fields,omitempty"`
	Expr       *Expr           `json:"expr,omitempty"`
	IterVar    string          `json:"iter_var,omitempty"`
	ThenSteps  []StepDecl      `json:"then_steps,omitempty"`
	ElseSteps  []StepDecl      `json:"else_steps,omitempty"`
	MatchArms  []MatchArmDecl  `json:"match_arms,omitempty"`
	Span       Span            `json:"span"`
}

type GivenSeedDecl struct {
	EntityName string          `json:"entity_name"`
	Fields     []FieldInitDecl `json:"fields"`
	Span       Span            `json:"span"`
}

type ExpectDecl struct {
	Left          Expr            `json:"left"`
	Op            string          `json:"op"`
	Right         Expr            `json:"right"`
	ErrorRaised   string          `json:"error_raised,omitempty"`
	EmittedEvent  string          `json:"emitted_event,omitempty"`
	EmittedFields []FieldInitDecl `json:"emitted_fields,omitempty"`
	StoredEntity  string          `json:"stored_entity,omitempty"`
	WhereField    string          `json:"where_field,omitempty"`
	WhereValue    *Expr           `json:"where_value,omitempty"`
	StoredFields  []FieldInitDecl `json:"stored_fields,omitempty"`
	NotStored     bool            `json:"not_stored,omitempty"`
	Span          Span            `json:"span"`
}

func walkExpr(e Expr, onExpr func(Expr)) {
	if onExpr != nil {
		onExpr(e)
	}
	if e.Left != nil {
		walkExpr(*e.Left, onExpr)
	}
	if e.Right != nil {
		walkExpr(*e.Right, onExpr)
	}
	for _, fi := range e.Fields {
		walkExpr(fi.Value, onExpr)
	}
	for _, it := range e.Items {
		walkExpr(it, onExpr)
	}
}

func walkStepsAndExprs(steps []StepDecl, onStep func(StepDecl), onExpr func(Expr)) {
	for _, s := range steps {
		if onStep != nil {
			onStep(s)
		}
		if s.Expr != nil {
			walkExpr(*s.Expr, onExpr)
		}
		if s.WhereValue != nil {
			walkExpr(*s.WhereValue, onExpr)
		}
		for _, arg := range s.PolicyArgs {
			walkExpr(arg, onExpr)
		}
		for _, fi := range s.Fields {
			walkExpr(fi.Value, onExpr)
		}
		walkStepsAndExprs(s.ThenSteps, onStep, onExpr)
		walkStepsAndExprs(s.ElseSteps, onStep, onExpr)
		for _, arm := range s.MatchArms {
			walkStepsAndExprs(arm.Steps, onStep, onExpr)
		}
	}
}

type ScenarioDecl struct {
	Name       string          `json:"name"`
	Givens     []GivenSeedDecl `json:"givens,omitempty"`
	WhenAction string          `json:"when_action"`
	WhenArgs   []Expr          `json:"when_args,omitempty"`
	Expects    []ExpectDecl    `json:"expects"`
	Span       Span            `json:"span"`
}

// Check scans a directory for .fdn files, parses them, and validates types and invariants.
func Check(workspaceDir string) (*SemanticIR, []Diagnostic) {
	files, err := findFDNFiles(workspaceDir)
	if err != nil {
		return nil, []Diagnostic{{
			Code:     "FDN0001",
			Severity: SeverityError,
			Message:  fmt.Sprintf("failed to scan workspace: %v", err),
			Span:     Span{File: workspaceDir, Line: 1, Column: 1},
		}}
	}
	if len(files) == 0 {
		return nil, []Diagnostic{{
			Code:     "FDN0002",
			Severity: SeverityError,
			Message:  "no .fdn Blueprint files found in workspace",
			Span:     Span{File: workspaceDir, Line: 1, Column: 1},
		}}
	}

	ir := &SemanticIR{}
	var diags []Diagnostic

	for _, f := range files {
		src, readErr := os.ReadFile(f)
		if readErr != nil {
			diags = append(diags, Diagnostic{
				Code:     "FDN0001",
				Severity: SeverityError,
				Message:  fmt.Sprintf("failed to read file: %v", readErr),
				Span:     Span{File: f, Line: 1, Column: 1},
			})
			continue
		}
		rel, relErr := filepath.Rel(workspaceDir, f)
		if relErr != nil {
			rel = filepath.Base(f)
		}
		rel = filepath.ToSlash(rel)

		var fileIR *SemanticIR
		var fileDiags []Diagnostic
		if strings.HasSuffix(rel, ".fdn.json") {
			fileIR = &SemanticIR{}
			if uErr := json.Unmarshal(src, fileIR); uErr != nil {
				fileDiags = append(fileDiags, Diagnostic{
					Code:     "FDN0006",
					Severity: SeverityError,
					Message:  fmt.Sprintf("failed to parse SemanticIR JSON: %v", uErr),
					Span:     Span{File: rel, Line: 1, Column: 1},
				})
				fileIR = nil
			}
		} else {
			fileIR, fileDiags = parseFile(rel, string(src))
		}
		diags = append(diags, fileDiags...)
		if fileIR != nil {
			ir.Enums = append(ir.Enums, fileIR.Enums...)
			ir.Values = append(ir.Values, fileIR.Values...)
			ir.Entities = append(ir.Entities, fileIR.Entities...)
			ir.Errors = append(ir.Errors, fileIR.Errors...)
			ir.Events = append(ir.Events, fileIR.Events...)
			ir.Policies = append(ir.Policies, fileIR.Policies...)
			ir.Actions = append(ir.Actions, fileIR.Actions...)
			ir.Scenarios = append(ir.Scenarios, fileIR.Scenarios...)
		}
	}

	if len(diags) > 0 {
		return nil, diags
	}

	valDiags := validateIR(ir)
	if len(valDiags) > 0 {
		return nil, valDiags
	}

	canonicalizeSemanticIR(ir)
	return ir, nil
}

func canonicalizeSemanticIR(ir *SemanticIR) {
	sort.SliceStable(ir.Enums, func(i, j int) bool { return ir.Enums[i].Name < ir.Enums[j].Name })
	sort.SliceStable(ir.Values, func(i, j int) bool { return ir.Values[i].Name < ir.Values[j].Name })
	sort.SliceStable(ir.Entities, func(i, j int) bool { return ir.Entities[i].Name < ir.Entities[j].Name })
	sort.SliceStable(ir.Errors, func(i, j int) bool { return ir.Errors[i].Name < ir.Errors[j].Name })
	sort.SliceStable(ir.Events, func(i, j int) bool { return ir.Events[i].Name < ir.Events[j].Name })
	sort.SliceStable(ir.Policies, func(i, j int) bool { return ir.Policies[i].Name < ir.Policies[j].Name })
	sort.SliceStable(ir.Actions, func(i, j int) bool { return ir.Actions[i].Name < ir.Actions[j].Name })
	sort.SliceStable(ir.Scenarios, func(i, j int) bool { return ir.Scenarios[i].Name < ir.Scenarios[j].Name })
}

func findFDNFiles(dir string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && strings.HasPrefix(d.Name(), ".") && path != dir {
			return filepath.SkipDir
		}
		if !d.IsDir() && (strings.HasSuffix(d.Name(), ".fdn") || strings.HasSuffix(d.Name(), ".fdn.json")) {
			out = append(out, path)
		}
		return nil
	})
	sort.Strings(out)
	return out, err
}

// IR validates all .fdn Blueprints in workspaceDir and returns the canonical JSON representation of SemanticIR.
func IR(workspaceDir string) ([]byte, []Diagnostic) {
	sem, diags := Check(workspaceDir)
	if len(diags) > 0 {
		return nil, diags
	}
	b, err := json.MarshalIndent(sem, "", "  ")
	if err != nil {
		return nil, []Diagnostic{{
			Code:     "FDN0005",
			Severity: SeverityError,
			Message:  fmt.Sprintf("failed to marshal SemanticIR: %v", err),
			Span:     Span{File: workspaceDir, Line: 1, Column: 1},
		}}
	}
	return append(b, '\n'), nil
}

// BuildOptions configures the compiler Build behavior.
type BuildOptions struct {
	Force bool
}

// BuildResult returns the validated SemanticIR and lowered ArchitectureIRs produced by Build.
type BuildResult struct {
	Semantic      *SemanticIR
	Profile       *BuildProfile
	Architectures []*ArchitectureIR
}

// Build compiles all .fdn Blueprints in workspaceDir according to foundation.build.yaml,
// lowers them into ArchitectureIR, reconciles .foundation-manifest.json, and emits target files.
func Build(workspaceDir string, opts BuildOptions) (*BuildResult, []Diagnostic) {
	sem, diags := Check(workspaceDir)
	if len(diags) > 0 {
		return nil, diags
	}

	bp, bpDiags := LoadBuildProfile(workspaceDir)
	if len(bpDiags) > 0 {
		return nil, bpDiags
	}

	res := &BuildResult{
		Semantic: sem,
		Profile:  bp,
	}

	reg := defaultRegistry()

	for _, target := range bp.Targets {
		arch, lowDiags := LowerArchitecture(sem, bp, target)
		if len(lowDiags) > 0 {
			return nil, lowDiags
		}
		res.Architectures = append(res.Architectures, arch)

		key := TargetKey{
			Kind:         target.Kind,
			Language:     target.Language,
			Architecture: target.Architecture,
		}
		emitter, ok := reg.emitters[key]
		if !ok {
			return nil, []Diagnostic{{
				Code:     "FDN0300",
				Severity: SeverityError,
				Message:  fmt.Sprintf("no emitter registered for target: kind=%s language=%s architecture=%s", target.Kind, target.Language, target.Architecture),
				Span:     Span{File: "foundation.build.yaml", Line: 1, Column: 1},
			}}
		}

		emission, emitDiags := emitter.Emit(arch)
		if len(emitDiags) > 0 {
			return nil, emitDiags
		}
		outDir := filepath.Join(workspaceDir, filepath.FromSlash(target.OutDir))
		recDiags := ReconcileManifest(outDir, emission.TrackedFiles, emission.StubFiles, opts.Force)
		if len(recDiags) > 0 {
			return nil, recDiags
		}
	}

	return res, nil
}

