package main

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/galihanggara68/foundation/pkg/emitterkit"
)

func main() {
	emitterkit.RunPlugin(generateRichDDD)
}

func generateRichDDD(req emitterkit.GenerationRequest) (emitterkit.GenerationResponse, error) {
	resp := emitterkit.GenerationResponse{
		Files: make(map[string]string),
		Stubs: map[string]string{
			"config.starter.json": "{\n  \"starter\": true\n}\n",
		},
		TestCommand: "go test ./...",
	}

	if req.SemanticIR == nil || len(req.SemanticIR.Entities) == 0 {
		return resp, nil
	}

	// Preserve existing files if provided
	for path, content := range req.Files {
		resp.Files[path] = content
	}

	for _, entity := range req.SemanticIR.Entities {
		fileName := fmt.Sprintf("internal/domain/%s_entity.go", strings.ToLower(entity.Name))
		testFileName := fmt.Sprintf("internal/domain/%s_aggregate_test.go", strings.ToLower(entity.Name))

		code := emitAggregateRoot(entity.Name, entity.Fields)
		testCode := emitAggregateTest(entity.Name, entity.Fields)

		resp.Files[fileName] = code
		resp.Files[testFileName] = testCode
	}

	return resp, nil
}

func emitAggregateRoot(name string, fields []emitterkit.FieldDecl) string {
	var sb strings.Builder
	sb.WriteString("package domain\n\n")

	hasTime := false
	firstStringField := ""
	for _, f := range fields {
		if f.Type == "timestamp" {
			hasTime = true
		}
		if firstStringField == "" && (f.Type == "string" || f.Type == "uuid") {
			firstStringField = emitterkit.ExportIdent(f.Name)
		}
	}

	if firstStringField != "" || hasTime {
		sb.WriteString("import (\n")
		if firstStringField != "" {
			sb.WriteString("\t\"errors\"\n")
		}
		if hasTime {
			sb.WriteString("\t\"time\"\n")
		}
		sb.WriteString(")\n\n")
	}

	// Struct definition with exported fields and JSON tags
	sb.WriteString(fmt.Sprintf("// %s is a rich DDD Aggregate Root with encapsulated invariants.\n", name))
	sb.WriteString(fmt.Sprintf("type %s struct {\n", name))
	for _, f := range fields {
		sb.WriteString(fmt.Sprintf("\t%s %s `json:%q`\n", emitterkit.ExportIdent(f.Name), goType(f.Type), f.Name))
	}
	sb.WriteString("}\n\n")

	// Constructor with invariant validation
	var paramList, assignList []string
	for _, f := range fields {
		pName := unexportIdent(f.Name)
		pType := goType(f.Type)
		paramList = append(paramList, fmt.Sprintf("%s %s", pName, pType))
		assignList = append(assignList, fmt.Sprintf("\t\t%s: %s,", emitterkit.ExportIdent(f.Name), pName))
	}

	sb.WriteString(fmt.Sprintf("// New%s constructs a new %s aggregate root and enforces business invariants.\n", name, name))
	sb.WriteString(fmt.Sprintf("func New%s(%s) (*%s, error) {\n", name, strings.Join(paramList, ", "), name))
	if firstStringField != "" {
		sb.WriteString(fmt.Sprintf("\tif %s == \"\" {\n", unexportIdent(firstStringField)))
		sb.WriteString(fmt.Sprintf("\t\treturn nil, errors.New(\"invariant violation: %s must not be empty\")\n", firstStringField))
		sb.WriteString("\t}\n")
	}
	sb.WriteString(fmt.Sprintf("\treturn &%s{\n%s\n\t}, nil\n", name, strings.Join(assignList, "\n")))
	sb.WriteString("}\n\n")

	// Invariant validation method
	sb.WriteString(fmt.Sprintf("// Validate verifies all aggregate invariants are satisfied.\n"))
	sb.WriteString(fmt.Sprintf("func (a *%s) Validate() error {\n", name))
	if firstStringField != "" {
		sb.WriteString(fmt.Sprintf("\tif a.%s == \"\" {\n\t\treturn errors.New(\"invariant violation: %s must not be empty\")\n\t}\n", firstStringField, firstStringField))
	}
	sb.WriteString("\treturn nil\n}\n\n")

	// Invariant mutation method
	if firstStringField != "" {
		sb.WriteString(fmt.Sprintf("// Set%s updates %s while enforcing domain invariants.\n", firstStringField, firstStringField))
		sb.WriteString(fmt.Sprintf("func (a *%s) Set%s(val string) error {\n", name, firstStringField))
		sb.WriteString(fmt.Sprintf("\tif val == \"\" {\n\t\treturn errors.New(\"invariant violation: %s must not be empty\")\n\t}\n", firstStringField))
		sb.WriteString(fmt.Sprintf("\ta.%s = val\n\treturn nil\n}\n", firstStringField))
	}

	return sb.String()
}

func emitAggregateTest(name string, fields []emitterkit.FieldDecl) string {
	var sb strings.Builder
	hasTime := false
	firstStringField := ""
	for _, f := range fields {
		if f.Type == "timestamp" {
			hasTime = true
		}
		if firstStringField == "" && (f.Type == "string" || f.Type == "uuid") {
			firstStringField = emitterkit.ExportIdent(f.Name)
		}
	}

	sb.WriteString("package domain\n\nimport (\n\t\"testing\"\n")
	if hasTime {
		sb.WriteString("\t\"time\"\n")
	}
	sb.WriteString(")\n\n")

	sb.WriteString(fmt.Sprintf("func Test%s_AggregateInvariants(t *testing.T) {\n", name))

	var args []string
	for _, f := range fields {
		switch f.Type {
		case "string", "uuid":
			args = append(args, `"test-val"`)
		case "int", "int64":
			args = append(args, "42")
		case "float":
			args = append(args, "3.14")
		case "bool":
			args = append(args, "true")
		case "timestamp":
			args = append(args, "time.Now()")
		default:
			args = append(args, `""`)
		}
	}

	sb.WriteString(fmt.Sprintf("\tagg, err := New%s(%s)\n", name, strings.Join(args, ", ")))
	sb.WriteString("\tif err != nil {\n\t\tt.Fatalf(\"failed to construct aggregate: %v\", err)\n\t}\n")
	sb.WriteString("\tif err := agg.Validate(); err != nil {\n\t\tt.Fatalf(\"aggregate validation failed: %v\", err)\n\t}\n")
	if firstStringField != "" {
		sb.WriteString(fmt.Sprintf("\tif err := agg.Set%s(\"\"); err == nil {\n\t\tt.Error(\"expected invariant violation error\")\n\t}\n", firstStringField))
	}
	sb.WriteString("}\n")
	return sb.String()
}

func unexportIdent(s string) string {
	exp := emitterkit.ExportIdent(s)
	if exp == "" {
		return ""
	}
	r := []rune(exp)
	r[0] = unicode.ToLower(r[0])
	return string(r)
}

func goType(fdnType string) string {
	switch fdnType {
	case "string", "uuid":
		return "string"
	case "int":
		return "int"
	case "int64":
		return "int64"
	case "float":
		return "float64"
	case "bool":
		return "bool"
	case "timestamp":
		return "time.Time"
	case "decimal":
		return "string"
	default:
		return fdnType
	}
}
