package tsclean

import (
	"fmt"
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
)

func emitTSPorts(archIR *arch.ArchitectureIR) string {
	var sb strings.Builder
	sb.WriteString(`import * as domain from "../entities/domain.ts";

export interface EventPublisher {
  publish(eventType: string, payload: unknown): Promise<void>;
  subscribe(eventType: string, handler: (payload: unknown) => Promise<void>): void;
}

export interface TransactionManager {
  withinTransaction<T>(fn: () => Promise<T>): Promise<T>;
}

export interface ActorResolverGateway {
  resolveActor(headers: Record<string, string | undefined>): Promise<Record<string, unknown>>;
}

`)

	for _, p := range archIR.Ports {
		sb.WriteString(fmt.Sprintf("export interface %s {\n", p.InterfaceName))
		for _, m := range p.Methods {
			switch m.Kind {
			case arch.RepoMethodCreate:
				sb.WriteString(fmt.Sprintf("  create(entity: Partial<domain.%s>): Promise<domain.%s>;\n", m.EntityName, m.EntityName))
			case arch.RepoMethodUpdate:
				sb.WriteString(fmt.Sprintf("  update(entity: domain.%s): Promise<void>;\n", m.EntityName))
			case arch.RepoMethodDelete:
				sb.WriteString(fmt.Sprintf("  %s(%s): Promise<void>;\n",
					toCamelCase(m.Name), formatTSRepoParams(m)))
			case arch.RepoMethodFind:
				sb.WriteString(fmt.Sprintf("  %s(%s): Promise<domain.%s | null>;\n",
					toCamelCase(m.Name), formatTSRepoParams(m), m.EntityName))
			case arch.RepoMethodFindMany:
				whereParams := formatTSRepoParams(m)
				callParams := ""
				if whereParams != "" {
					callParams = whereParams + ", "
				}
				callParams += "cursor: string, limit: number, direction: string"
				sb.WriteString(fmt.Sprintf("  %s(%s): Promise<{ items: domain.%s[]; page_info: domain.PageInfo }>;\n",
					toCamelCase(m.Name), callParams, m.EntityName))
			}
		}
		sb.WriteString("}\n\n")
	}

	for _, sl := range archIR.Slots {
		sb.WriteString(fmt.Sprintf("export interface %sSlot {\n", sl.Name))
		for _, m := range sl.Methods {
			var params []string
			for _, p := range m.Params {
				params = append(params, fmt.Sprintf("%s: %s", p.Name, tsType(p.Type, false)))
			}
			retTS := tsType(m.ReturnType, false)
			sb.WriteString(fmt.Sprintf("  %s(%s): Promise<%s>;\n", toCamelCase(m.Name), strings.Join(params, ", "), retTS))
		}
		sb.WriteString("}\n\n")
	}

	return sb.String()
}
