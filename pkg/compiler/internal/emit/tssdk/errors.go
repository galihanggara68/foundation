package tssdk

import (
	"fmt"
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/platform"
)

func emitErrors(archIR *arch.ArchitectureIR) string {
	var sb strings.Builder
	sb.WriteString(`export class FoundationError extends Error {
  constructor(message: string) {
    super(message);
    this.name = this.constructor.name;
  }
}

export class ApiError extends FoundationError {
  readonly statusCode: number;
  readonly code: string;
  readonly kind: string;

  constructor(statusCode: number, code: string, kind: string, message: string) {
    super(message || ` + "`API error (${statusCode} ${code})`" + `);
    this.name = this.constructor.name;
    this.statusCode = statusCode;
    this.code = code;
    this.kind = kind;
  }
}

`)

	kinds := platform.StandardErrorKinds()
	nameForKind := make(map[string]string, len(kinds))
	for _, name := range platform.ReservedErrorNames() {
		if kind, ok := platform.ReservedErrorName(name); ok {
			nameForKind[string(kind)] = name
		}
	}
	for _, kind := range kinds {
		sb.WriteString(fmt.Sprintf("export class %sError extends ApiError {}\n", nameForKind[string(kind)]))
	}
	sb.WriteString("\n")

	for _, dErr := range archIR.DomainErrors {
		parent := "ApiError"
		if name, standard := nameForKind[string(dErr.Kind)]; standard {
			parent = name + "Error"
		}
		sb.WriteString(fmt.Sprintf("// %sError represents domain error %s (%s).\nexport class %sError extends %s {}\n\n",
			dErr.Name, dErr.Name, dErr.Kind, dErr.Name, parent))
	}

	sb.WriteString(`export function createError(status: number, data: any): ApiError {
  const code = (data && typeof data.code === "string") ? data.code : ` + "`HTTP_${status}`" + `;
  const kind = (data && typeof data.kind === "string") ? data.kind : defaultKindForStatus(status);
  const message = (data && typeof data.message === "string") ? data.message : "";

  switch (code) {
`)
	for _, dErr := range archIR.DomainErrors {
		sb.WriteString(fmt.Sprintf("    case %q:\n      return new %sError(status, code, kind, message);\n", dErr.Name, dErr.Name))
	}
	sb.WriteString(`  }

  switch (kind) {
`)
	for _, kind := range kinds {
		sb.WriteString(fmt.Sprintf("    case %q:\n      return new %sError(status, code, kind, message);\n", string(kind), nameForKind[string(kind)]))
	}
	sb.WriteString(`    default:
      return new ApiError(status, code, kind, message);
  }
}

function defaultKindForStatus(status: number): string {
  switch (status) {
`)

	byStatus := make(map[int][]string, len(kinds))
	for _, kind := range kinds {
		policy, _ := platform.ErrorPolicyFor(kind)
		byStatus[policy.Status] = append(byStatus[policy.Status], string(kind))
	}
	seen := make(map[int]bool, len(byStatus))
	for _, kind := range kinds {
		policy, _ := platform.ErrorPolicyFor(kind)
		if seen[policy.Status] {
			continue
		}
		seen[policy.Status] = true
		if tie := byStatus[policy.Status]; len(tie) > 1 {
			sb.WriteString(fmt.Sprintf("    // %d is answered by %s; %s wins the tie-break.\n",
				policy.Status, strings.Join(tie, " and "), kind))
		}
		sb.WriteString(fmt.Sprintf("    case %d: return %q;\n", policy.Status, kind))
	}
	sb.WriteString(`    default: return "internal";
  }
}
`)

	return sb.String()
}
