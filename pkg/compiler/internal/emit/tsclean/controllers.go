package tsclean

import (
	"fmt"
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
)

func emitTSExpressController(archIR *arch.ArchitectureIR) string {
	var sb strings.Builder
	sb.WriteString("import * as domain from \"../entities/domain.ts\";\n")
	for _, uc := range archIR.UseCases {
		sb.WriteString(fmt.Sprintf("import { %s } from \"../usecases/%s_usecase.ts\";\n", uc.Name, strings.ToLower(uc.ActionName)))
	}

	sb.WriteString(`
export interface ExpressRequest {
  headers: Record<string, string | undefined>;
  params: Record<string, string>;
  body: unknown;
}

export interface ExpressResponse {
  status(code: number): ExpressResponse;
  json(payload: unknown): void;
  setHeader(name: string, value: string): void;
}

export interface ExpressRouterLike {
  get(path: string, handler: (req: ExpressRequest, res: ExpressResponse) => Promise<void>): void;
  post(path: string, handler: (req: ExpressRequest, res: ExpressResponse) => Promise<void>): void;
  put(path: string, handler: (req: ExpressRequest, res: ExpressResponse) => Promise<void>): void;
  delete(path: string, handler: (req: ExpressRequest, res: ExpressResponse) => Promise<void>): void;
}

function writeError(res: ExpressResponse, err: unknown): void {
  if (err instanceof domain.DomainError) {
    let status = 500;
    switch (err.kind) {
      case "not_found":
        status = 404;
        break;
      case "validation":
        status = 400;
        break;
      case "conflict":
        status = 409;
        break;
      case "unauthorized":
        status = 401;
        break;
      case "forbidden":
        status = 403;
        break;
    }
    res.status(status).json({ code: err.code, kind: err.kind, message: err.message });
    return;
  }
  res.status(400).json({ error: String(err) });
}

export class ExpressController {
`)

	for _, uc := range archIR.UseCases {
		sb.WriteString(fmt.Sprintf("  readonly %s: %s;\n", toCamelCase(uc.ActionName), uc.Name))
	}
	sb.WriteString("\n  constructor(\n")
	var ctorParams []string
	for _, uc := range archIR.UseCases {
		ctorParams = append(ctorParams, fmt.Sprintf("    %s: %s", toCamelCase(uc.ActionName), uc.Name))
	}
	sb.WriteString(strings.Join(ctorParams, ",\n") + "\n  ) {\n")
	for _, uc := range archIR.UseCases {
		c := toCamelCase(uc.ActionName)
		sb.WriteString(fmt.Sprintf("    this.%s = %s;\n", c, c))
	}
	sb.WriteString("  }\n\n  registerRoutes(router: ExpressRouterLike): void {\n")
	for _, h := range archIR.HTTPHandlers {
		expressPath := strings.ReplaceAll(h.Path, "{", ":")
		expressPath = strings.ReplaceAll(expressPath, "}", "")
		method := strings.ToLower(h.Method)
		sb.WriteString(fmt.Sprintf("    router.%s(%q, (req, res) => this.handle%s(req, res));\n", method, expressPath, h.ActionName))
	}
	sb.WriteString("  }\n\n")

	for _, h := range archIR.HTTPHandlers {
		sb.WriteString(fmt.Sprintf("  async handle%s(req: ExpressRequest, res: ExpressResponse): Promise<void> {\n", h.ActionName))
		sb.WriteString(`    const actor: domain.Actor = {
      id: req.headers["x-actor-id"] ?? "",
      role: req.headers["x-actor-role"] ?? "",
      is_verified: req.headers["x-actor-verified"] === "true",
    };
    try {
`)
		pathParamSet := make(map[string]bool)
		for _, pp := range h.PathParams {
			pathParamSet[pp] = true
		}
		var callArgs []string
		callArgs = append(callArgs, "actor")
		for _, p := range h.Params {
			if pathParamSet[p.Name] {
				sb.WriteString(fmt.Sprintf("      const %s = req.params[%q] as %s;\n", p.Name, p.Name, tsType(p.Type, false)))
			} else {
				sb.WriteString(fmt.Sprintf("      const %s = req.body as %s;\n", p.Name, tsType(p.Type, false)))
			}
			callArgs = append(callArgs, p.Name)
		}
		if h.Method != "GET" && h.ExpectedVersionParam != "" {
			sb.WriteString(fmt.Sprintf("      if (req.headers[\"if-match\"] && req.headers[\"if-match\"] !== \"*\") {\n        const cleanMatch = req.headers[\"if-match\"].replace(/\"/g, \"\");\n        if (String(%s) !== cleanMatch) {\n          throw domain.ErrConflict;\n        }\n      }\n", h.ExpectedVersionParam))
		}

		var targetUC *arch.UseCaseIR
		for i := range archIR.UseCases {
			if archIR.UseCases[i].ActionName == h.ActionName {
				targetUC = &archIR.UseCases[i]
				break
			}
		}

		status := 200
		if h.Method == "POST" {
			status = 201
		}
		sb.WriteString(fmt.Sprintf("      const out = await this.%s.execute(%s);\n",
			toCamelCase(h.ActionName), strings.Join(callArgs, ", ")))
		if h.Method == "GET" && targetUC != nil {
			retEnt := archIR.EntityByName(targetUC.ReturnType)
			if vField, _, hasV := arch.EntityVersionField(retEnt); hasV {
				sb.WriteString(fmt.Sprintf("      if (res.setHeader && (out as any)?.%s !== undefined) {\n        res.setHeader(\"ETag\", `\"${(out as any).%s}\"`);\n      }\n", vField, vField))
			}
		}
		sb.WriteString(fmt.Sprintf("      res.status(%d).json(out);\n    } catch (err) {\n      writeError(res, err);\n    }\n  }\n\n", status))
	}

	sb.WriteString("}\n")
	return sb.String()
}

func emitTSContainer(archIR *arch.ArchitectureIR) string {
	var sb strings.Builder
	sb.WriteString(`import * as memory from "../gateways/memory_repositories.ts";
import * as postgres from "../gateways/postgres_repositories.ts";
import { ExpressController } from "../controllers/express_router.ts";
import { createMemoryDriver, createPostgresDriver } from "@foundation-contrib/runtime";
`)
	for _, sl := range archIR.Slots {
		sb.WriteString(fmt.Sprintf("import { create%sSlot } from \"../slots/%s_stub.ts\";\n", sl.Name, strings.ToLower(sl.Name)))
	}
	for _, uc := range archIR.UseCases {
		sb.WriteString(fmt.Sprintf("import { %s } from \"../usecases/%s_usecase.ts\";\n", uc.Name, strings.ToLower(uc.ActionName)))
	}

	sb.WriteString("\nexport function createTestContainer() {\n")
	sb.WriteString("  const runtimeDriver = createMemoryDriver();\n")
	var snapRepos []string
	for _, p := range archIR.Ports {
		rVar := toCamelCase(p.EntityName) + "Repo"
		sb.WriteString(fmt.Sprintf("  const %s = new memory.InMemory%sRepository();\n", rVar, p.EntityName))
		snapRepos = append(snapRepos, rVar)
	}
	for _, sl := range archIR.Slots {
		sb.WriteString(fmt.Sprintf("  const %sSlot = new memory.Fake%sSlot(create%sSlot());\n",
			toCamelCase(sl.Name), sl.Name, sl.Name))
	}
	sb.WriteString("  const events = new memory.InMemoryEventPublisher();\n")
	sb.WriteString(fmt.Sprintf("  const txManager = new memory.InMemoryTransactionManager(%s);\n", strings.Join(snapRepos, ", ")))

	var ctrlArgs []string
	for _, uc := range archIR.UseCases {
		var depArgs []string
		for _, dep := range uc.RepoDeps {
			depArgs = append(depArgs, toCamelCase(dep)+"Repo")
		}
		for _, sDep := range uc.SlotDeps {
			depArgs = append(depArgs, toCamelCase(sDep)+"Slot")
		}
		if uc.EmitsEvents {
			depArgs = append(depArgs, "events")
		}
		if _, isTx := uc.HasTrait("transactional"); isTx {
			depArgs = append(depArgs, "txManager")
		}
		if _, isIdem := uc.HasTrait("idempotent"); isIdem {
			depArgs = append(depArgs, "runtimeDriver.idempotency()")
		}
		if _, isCache := uc.HasTrait("cache"); isCache {
			depArgs = append(depArgs, "runtimeDriver.cache()")
		}
		if _, isOutbox := uc.HasTrait("outbox"); isOutbox {
			depArgs = append(depArgs, "runtimeDriver.outbox()")
		}
		ucVar := toCamelCase(uc.ActionName)
		sb.WriteString(fmt.Sprintf("  const %s = new %s(%s);\n", ucVar, uc.Name, strings.Join(depArgs, ", ")))
		ctrlArgs = append(ctrlArgs, ucVar)
	}
	for _, eh := range archIR.EventHandlers {
		sb.WriteString(fmt.Sprintf("  events.subscribe(%q, async (payload) => {\n    await %s.execute({ id: \"system\", role: \"system\", is_verified: true }, payload as any);\n  });\n", eh.EventName, toCamelCase(eh.ActionName)))
	}
	sb.WriteString(fmt.Sprintf("  const controller = new ExpressController(%s);\n", strings.Join(ctrlArgs, ", ")))
	sb.WriteString("  return {\n")
	for _, p := range archIR.Ports {
		sb.WriteString(fmt.Sprintf("    %sRepo,\n", toCamelCase(p.EntityName)))
	}
	for _, sl := range archIR.Slots {
		sb.WriteString(fmt.Sprintf("    %sSlot,\n", toCamelCase(sl.Name)))
	}
	sb.WriteString("    events,\n    txManager,\n")
	for _, uc := range archIR.UseCases {
		sb.WriteString(fmt.Sprintf("    %s,\n", toCamelCase(uc.ActionName)))
	}
	sb.WriteString("    controller,\n  };\n}\n\n")

	sb.WriteString("export function createPostgresContainer(db: postgres.PgQueryable) {\n")
	sb.WriteString("  const runtimeDriver = createPostgresDriver(db);\n  void runtimeDriver;\n")
	for _, p := range archIR.Ports {
		rVar := toCamelCase(p.EntityName) + "Repo"
		sb.WriteString(fmt.Sprintf("  const %s = new postgres.Postgres%sRepository(db);\n  void %s;\n", rVar, p.EntityName, rVar))
	}
	for _, sl := range archIR.Slots {
		sVar := toCamelCase(sl.Name) + "Slot"
		sb.WriteString(fmt.Sprintf("  const %s = create%sSlot();\n  void %s;\n", sVar, sl.Name, sVar))
	}
	sb.WriteString("  const events = new memory.InMemoryEventPublisher();\n  void events;\n")
	sb.WriteString("  const txManager = new postgres.PostgresTransactionManager(db);\n  void txManager;\n")
	var pgCtrlArgs []string
	for _, uc := range archIR.UseCases {
		var depArgs []string
		for _, dep := range uc.RepoDeps {
			depArgs = append(depArgs, toCamelCase(dep)+"Repo")
		}
		for _, sDep := range uc.SlotDeps {
			depArgs = append(depArgs, toCamelCase(sDep)+"Slot")
		}
		if uc.EmitsEvents {
			depArgs = append(depArgs, "events")
		}
		if _, isTx := uc.HasTrait("transactional"); isTx {
			depArgs = append(depArgs, "txManager")
		}
		if _, isIdem := uc.HasTrait("idempotent"); isIdem {
			depArgs = append(depArgs, "runtimeDriver.idempotency()")
		}
		if _, isCache := uc.HasTrait("cache"); isCache {
			depArgs = append(depArgs, "runtimeDriver.cache()")
		}
		if _, isOutbox := uc.HasTrait("outbox"); isOutbox {
			depArgs = append(depArgs, "runtimeDriver.outbox()")
		}
		ucVar := toCamelCase(uc.ActionName)
		sb.WriteString(fmt.Sprintf("  const %s = new %s(%s);\n", ucVar, uc.Name, strings.Join(depArgs, ", ")))
		pgCtrlArgs = append(pgCtrlArgs, ucVar)
	}
	for _, eh := range archIR.EventHandlers {
		sb.WriteString(fmt.Sprintf("  events.subscribe(%q, async (payload) => {\n    await %s.execute({ id: \"system\", role: \"system\", is_verified: true }, payload as any);\n  });\n", eh.EventName, toCamelCase(eh.ActionName)))
	}
	sb.WriteString(fmt.Sprintf("  const controller = new ExpressController(%s);\n", strings.Join(pgCtrlArgs, ", ")))
	sb.WriteString("  return { controller };\n}\n")

	return sb.String()
}
