package tssdk

import (
	"fmt"
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
)

func emitTypes(archIR *arch.ArchitectureIR) string {
	var sb strings.Builder
	sb.WriteString(`export interface PageInfo {
  next_cursor?: string;
  prev_cursor?: string;
  has_next: boolean;
  has_prev: boolean;
}

`)

	// Emit Enums
	for _, en := range archIR.DomainEnums {
		var variants []string
		for _, v := range en.Variants {
			variants = append(variants, fmt.Sprintf("%q", v))
		}
		sb.WriteString(fmt.Sprintf("// %s enum representation.\nexport type %s = %s;\n\n",
			en.Name, en.Name, strings.Join(variants, " | ")))
	}

	// Emit Values
	for _, val := range archIR.DomainValues {
		sb.WriteString(fmt.Sprintf("// %s value object.\nexport interface %s {\n", val.Name, val.Name))
		for _, f := range val.Fields {
			fName := arch.ToSnakeCase(f.Name)
			fType := toTSType(f.Type)
			sb.WriteString(fmt.Sprintf("  %s: %s;\n", fName, fType))
		}
		sb.WriteString("}\n\n")
	}

	// Emit Entities
	for _, ent := range archIR.DomainEntities {
		sb.WriteString(fmt.Sprintf("// %s entity.\nexport interface %s {\n", ent.Name, ent.Name))
		for _, f := range ent.Fields {
			fName := arch.ToSnakeCase(f.Name)
			fType := toTSType(f.Type)
			sb.WriteString(fmt.Sprintf("  %s: %s;\n", fName, fType))
		}
		sb.WriteString("}\n\n")
	}

	// Emit Action Inputs & Responses
	for _, h := range archIR.HTTPHandlers {
		inputName := h.ActionName + "Input"
		if !archIR.HasTypeNamed(inputName) {
			sb.WriteString(fmt.Sprintf("// %sInput holds request parameters for %s.\nexport interface %sInput {\n",
				h.ActionName, h.ActionName, h.ActionName))
			for _, p := range h.Params {
				pName := arch.ToSnakeCase(p.Name)
				pType := toTSType(p.Type)
				sb.WriteString(fmt.Sprintf("  %s: %s;\n", pName, pType))
			}
			sb.WriteString("}\n\n")
		}

		if h.IsListReturning {
			itemType := toTSType(strings.TrimPrefix(h.ReturnType, "[]"))
			sb.WriteString(fmt.Sprintf("// %sResponse contains the paginated result for %s.\nexport interface %sResponse {\n",
				h.ActionName, h.ActionName, h.ActionName))
			sb.WriteString(fmt.Sprintf("  items: %s[];\n", itemType))
			sb.WriteString("  page_info: PageInfo;\n")
			sb.WriteString("}\n\n")
		}
	}

	return sb.String()
}

func toTSType(t string) string {
	if strings.HasPrefix(t, "[]") {
		return toTSType(strings.TrimPrefix(t, "[]")) + "[]"
	}
	switch t {
	case "string", "uuid", "timestamp", "decimal":
		return "string"
	case "int", "float":
		return "number"
	case "bool":
		return "boolean"
	default:
		return t
	}
}
