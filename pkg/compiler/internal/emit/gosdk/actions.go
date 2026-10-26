package gosdk

import (
	"fmt"
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
)

func emitActions(archIR *arch.ArchitectureIR) string {
	var sb strings.Builder
	sb.WriteString(`package client

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

var (
	_ = fmt.Sprintf
	_ = strings.ReplaceAll
	_ = strconv.Itoa
)

`)

	for _, h := range archIR.HTTPHandlers {
		methodName := arch.ExportIdent(h.ActionName)
		inputTypeName := h.ActionName + "Input"

		isVoid := h.ReturnType == "" || h.ReturnType == "void"
		isList := h.IsListReturning

		var retSig string
		if isVoid {
			retSig = "error"
		} else if isList {
			retSig = fmt.Sprintf("(*%sResponse, error)", h.ActionName)
		} else {
			retSig = fmt.Sprintf("(*%s, error)", toGoType(h.ReturnType))
		}

		sb.WriteString(fmt.Sprintf("// %s executes the %s action.\n", methodName, h.ActionName))
		sb.WriteString(fmt.Sprintf("func (c *Client) %s(ctx context.Context, input %s) %s {\n", methodName, inputTypeName, retSig))

		// Build path substitution
		sb.WriteString(fmt.Sprintf("\treqPath := %q\n", h.Path))
		pathParamSet := make(map[string]bool)
		for _, pp := range h.PathParams {
			pathParamSet[pp] = true
			fieldIdent := arch.ExportIdent(pp)
			sb.WriteString(fmt.Sprintf("\treqPath = strings.ReplaceAll(reqPath, \"{%s}\", url.PathEscape(fmt.Sprintf(\"%%v\", input.%s)))\n", pp, fieldIdent))
		}

		// Handle query and body
		if h.Method == "GET" {
			sb.WriteString("\tquery := make(url.Values)\n")
			for _, p := range h.Params {
				if pathParamSet[p.Name] {
					continue
				}
				fieldIdent := arch.ExportIdent(p.Name)
				switch p.Type {
				case "string", "uuid":
					sb.WriteString(fmt.Sprintf("\tif input.%s != \"\" {\n\t\tquery.Set(%q, input.%s)\n\t}\n", fieldIdent, p.Name, fieldIdent))
				case "int":
					sb.WriteString(fmt.Sprintf("\tif input.%s != 0 {\n\t\tquery.Set(%q, strconv.Itoa(input.%s))\n\t}\n", fieldIdent, p.Name, fieldIdent))
				case "bool":
					sb.WriteString(fmt.Sprintf("\tquery.Set(%q, strconv.FormatBool(input.%s))\n", p.Name, fieldIdent))
				default:
					sb.WriteString(fmt.Sprintf("\tif input.%s != \"\" {\n\t\tquery.Set(%q, fmt.Sprintf(\"%%v\", input.%s))\n\t}\n", fieldIdent, p.Name, fieldIdent))
				}
			}
			sb.WriteString("\tvar body interface{} = nil\n")
		} else {
			sb.WriteString("\tvar query url.Values = nil\n")
			sb.WriteString("\tbody := input\n")
		}

		if isVoid {
			sb.WriteString(fmt.Sprintf("\treturn c.doRequest(ctx, %q, reqPath, query, body, nil)\n}\n\n", h.Method))
		} else if isList {
			sb.WriteString(fmt.Sprintf("\tvar res %sResponse\n", h.ActionName))
			sb.WriteString(fmt.Sprintf("\tif err := c.doRequest(ctx, %q, reqPath, query, body, &res); err != nil {\n\t\treturn nil, err\n\t}\n", h.Method))
			sb.WriteString("\treturn &res, nil\n}\n\n")
		} else {
			retType := toGoType(h.ReturnType)
			sb.WriteString(fmt.Sprintf("\tvar res %s\n", retType))
			sb.WriteString(fmt.Sprintf("\tif err := c.doRequest(ctx, %q, reqPath, query, body, &res); err != nil {\n\t\treturn nil, err\n\t}\n", h.Method))
			sb.WriteString("\treturn &res, nil\n}\n\n")
		}
	}

	return sb.String()
}
