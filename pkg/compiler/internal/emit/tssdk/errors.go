package tssdk

import (
	"fmt"
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
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

export class UnauthenticatedError extends ApiError {}
export class ForbiddenError extends ApiError {}
export class NotFoundError extends ApiError {}
export class ConflictError extends ApiError {}
export class ValidationError extends ApiError {}

`)

	for _, dErr := range archIR.DomainErrors {
		parent := "ApiError"
		switch dErr.Kind {
		case "unauthenticated":
			parent = "UnauthenticatedError"
		case "forbidden":
			parent = "ForbiddenError"
		case "not_found":
			parent = "NotFoundError"
		case "conflict":
			parent = "ConflictError"
		case "validation":
			parent = "ValidationError"
		case "internal":
			parent = "InternalError"
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
    case "unauthenticated":
      return new UnauthenticatedError(status, code, kind, message);
    case "forbidden":
      return new ForbiddenError(status, code, kind, message);
    case "not_found":
      return new NotFoundError(status, code, kind, message);
    case "conflict":
      return new ConflictError(status, code, kind, message);
    case "validation":
      return new ValidationError(status, code, kind, message);
    default:
      return new ApiError(status, code, kind, message);
  }
}

function defaultKindForStatus(status: number): string {
  switch (status) {
    case 401: return "unauthenticated";
    case 403: return "forbidden";
    case 404: return "not_found";
    case 409: return "conflict";
    case 400: return "validation";
    default: return "internal";
  }
}
`)

	return sb.String()
}
