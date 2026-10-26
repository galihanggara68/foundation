package gosdk

import (
	"fmt"
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
)

func emitTypes(archIR *arch.ArchitectureIR) string {
	var sb strings.Builder
	sb.WriteString(`package client

import (
	"time"
)

var _ = time.RFC3339

// PageInfo represents keyset pagination cursor and navigation metadata.
type PageInfo struct {
	NextCursor string ` + "`json:\"next_cursor,omitempty\"`" + `
	PrevCursor string ` + "`json:\"prev_cursor,omitempty\"`" + `
	HasNext    bool   ` + "`json:\"has_next\"`" + `
	HasPrev    bool   ` + "`json:\"has_prev\"`" + `
}

`)

	// Emit Enums
	for _, en := range archIR.DomainEnums {
		sb.WriteString(fmt.Sprintf("// %s represents the %s enum.\ntype %s string\n\nconst (\n", en.Name, en.Name, en.Name))
		for _, v := range en.Variants {
			varIdent := arch.ExportIdent(v)
			sb.WriteString(fmt.Sprintf("\t%s%s %s = %q\n", en.Name, varIdent, en.Name, v))
		}
		sb.WriteString(")\n\n")
	}

	// Emit Values
	for _, val := range archIR.DomainValues {
		sb.WriteString(fmt.Sprintf("// %s represents the value object %s.\ntype %s struct {\n", val.Name, val.Name, val.Name))
		for _, f := range val.Fields {
			fName := arch.ExportIdent(f.Name)
			fType := toGoType(f.Type)
			sb.WriteString(fmt.Sprintf("\t%s %s `json:%q`\n", fName, fType, f.Name))
		}
		sb.WriteString("}\n\n")
	}

	// Emit Entities
	for _, ent := range archIR.DomainEntities {
		sb.WriteString(fmt.Sprintf("// %s represents the %s entity.\ntype %s struct {\n", ent.Name, ent.Name, ent.Name))
		for _, f := range ent.Fields {
			fName := arch.ExportIdent(f.Name)
			fType := toGoType(f.Type)
			sb.WriteString(fmt.Sprintf("\t%s %s `json:%q`\n", fName, fType, f.Name))
		}
		sb.WriteString("}\n\n")
	}

	// Emit Action Inputs
	for _, h := range archIR.HTTPHandlers {
		inputName := h.ActionName + "Input"
		if !archIR.HasTypeNamed(inputName) {
			sb.WriteString(fmt.Sprintf("// %sInput holds the request parameters for %s.\ntype %sInput struct {\n", h.ActionName, h.ActionName, h.ActionName))
			for _, p := range h.Params {
				pName := arch.ExportIdent(p.Name)
				pType := toGoType(p.Type)
				sb.WriteString(fmt.Sprintf("\t%s %s `json:%q`\n", pName, pType, p.Name))
			}
			sb.WriteString("}\n\n")
		}

		// If list-returning, emit ActionResponse
		if h.IsListReturning {
			itemType := toGoType(strings.TrimPrefix(h.ReturnType, "[]"))
			sb.WriteString(fmt.Sprintf("// %sResponse contains the paginated result for %s.\ntype %sResponse struct {\n", h.ActionName, h.ActionName, h.ActionName))
			sb.WriteString(fmt.Sprintf("\tItems    []%s     `json:\"items\"`\n", itemType))
			sb.WriteString("\tPageInfo PageInfo `json:\"page_info\"`\n")
			sb.WriteString("}\n\n")
		}
	}

	return sb.String()
}

func toGoType(t string) string {
	if strings.HasPrefix(t, "[]") {
		return "[]" + toGoType(strings.TrimPrefix(t, "[]"))
	}
	switch t {
	case "string", "uuid":
		return "string"
	case "int":
		return "int"
	case "float":
		return "float64"
	case "bool":
		return "bool"
	case "timestamp":
		return "time.Time"
	case "decimal":
		return "string"
	default:
		return arch.ExportIdent(t)
	}
}
