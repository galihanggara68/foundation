package tsclean

import (
	"fmt"
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/emit"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

// Emitter implements emit.TargetEmitter for TypeScript Clean Architecture.
type Emitter struct{}

func (Emitter) Emit(archIR *arch.ArchitectureIR) (emit.Emission, []ir.Diagnostic) {
	tracked := make(map[string][]byte)
	stubs := make(map[string][]byte)

	pkgName := archIR.Module
	if pkgName == "" {
		pkgName = "foundation-ts-backend"
	}

	tracked["package.json"] = []byte(fmt.Sprintf(`{
  "name": %q,
  "version": "0.1.0",
  "private": true,
  "type": "module",
  "scripts": {
    "typecheck": "tsc --noEmit",
    "test": "node --test test/*.test.ts"
  },
  "dependencies": {
    "express": "^4.21.0",
    "pg": "^8.13.0"
  },
  "devDependencies": {
    "@types/express": "^4.17.21",
    "@types/node": "^22.7.0",
    "@types/pg": "^8.11.10",
    "typescript": "^5.6.0"
  }
}
`, pkgName))

	tracked["tsconfig.json"] = []byte(`{
  "compilerOptions": {
    "target": "ES2022",
    "module": "NodeNext",
    "moduleResolution": "NodeNext",
    "strict": true,
    "noEmit": true,
    "esModuleInterop": true,
    "allowImportingTsExtensions": true,
    "skipLibCheck": true
  },
  "include": ["src/**/*.ts", "test/**/*.ts"]
}
`)

	if archIR.Storage.SchemaSQL != "" {
		tracked["schema.sql"] = []byte(archIR.Storage.SchemaSQL)
	}
	for _, mig := range archIR.Storage.Migrations {
		tracked[fmt.Sprintf("migrations/%s.up.sql", mig.Name)] = []byte(mig.UpSQL)
		tracked[fmt.Sprintf("migrations/%s.down.sql", mig.Name)] = []byte(mig.DownSQL)
	}

	tracked["src/entities/domain.ts"] = []byte(emitTSDomain(archIR))
	tracked["src/usecases/ports.ts"] = []byte(emitTSPorts(archIR))

	for _, uc := range archIR.UseCases {
		relPath := fmt.Sprintf("src/usecases/%s_usecase.ts", strings.ToLower(uc.ActionName))
		tracked[relPath] = []byte(emitTSUseCase(archIR, uc))
	}

	for _, sl := range archIR.Slots {
		stubPath := fmt.Sprintf("src/slots/%s_stub.ts", strings.ToLower(sl.Name))
		stubs[stubPath] = []byte(emitTSSlotStub(archIR, sl))
	}

	tracked["src/gateways/memory_repositories.ts"] = []byte(emitTSMemoryGateways(archIR))
	tracked["src/gateways/postgres_repositories.ts"] = []byte(emitTSPostgresGateways(archIR))
	tracked["src/controllers/express_router.ts"] = []byte(emitTSExpressController(archIR))
	tracked["src/main/container.ts"] = []byte(emitTSContainer(archIR))
	tracked["src/main/server.ts"] = []byte(emitTSServer())
	tracked["test/contracts.test.ts"] = []byte(emitTSContractsTest(archIR))
	tracked["test/scenarios.test.ts"] = []byte(emitTSScenariosTest(archIR))

	return emit.Emission{
		TrackedFiles: tracked,
		StubFiles:    stubs,
	}, nil
}

func tsType(fdnType string, inDomainFile bool) string {
	if strings.HasPrefix(fdnType, "[]") {
		return tsType(strings.TrimPrefix(fdnType, "[]"), inDomainFile) + "[]"
	}
	switch fdnType {
	case "uuid", "string", "timestamp":
		return "string"
	case "int", "decimal":
		return "number"
	case "bool":
		return "boolean"
	case "":
		return "void"
	default:
		if inDomainFile {
			return fdnType
		}
		return "domain." + fdnType
	}
}

func tsZeroVal(fdnType string, archIR *arch.ArchitectureIR, inDomainFile bool) string {
	if strings.HasPrefix(fdnType, "[]") {
		return "[]"
	}
	switch fdnType {
	case "uuid", "string", "timestamp":
		return `""`
	case "int", "decimal":
		return "0"
	case "bool":
		return "false"
	}
	for _, en := range archIR.DomainEnums {
		if en.Name == fdnType && len(en.Variants) > 0 {
			return fmt.Sprintf("%q", en.Variants[0])
		}
	}
	for _, v := range archIR.DomainValues {
		if v.Name == fdnType {
			var parts []string
			for _, f := range v.Fields {
				parts = append(parts, fmt.Sprintf("%s: %s", f.Name, tsZeroVal(f.Type, archIR, inDomainFile)))
			}
			return "{ " + strings.Join(parts, ", ") + " }"
		}
	}
	for _, e := range archIR.DomainEntities {
		if e.Name == fdnType {
			var parts []string
			for _, f := range e.Fields {
				parts = append(parts, fmt.Sprintf("%s: %s", f.Name, tsZeroVal(f.Type, archIR, inDomainFile)))
			}
			return "{ " + strings.Join(parts, ", ") + " }"
		}
	}
	return fmt.Sprintf("({} as %s)", tsType(fdnType, inDomainFile))
}

func toCamelCase(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToLower(s[:1]) + s[1:]
}
