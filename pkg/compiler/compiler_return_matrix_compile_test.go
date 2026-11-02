package compiler_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler"
)

// TestBuild_ReturnTypeMatrix_EmitsCompilableGo compiles a hexagon whose actions return
// every shape the Go emitter distinguishes: an entity (by identity), a value, an enum,
// a scalar, and lists of each — including a timestamp-carrying value, which is the only
// way time.Time reaches the domain values file. compiler.Build only runs gofmt, so the
// emitted module is compiled here; that is the check gofmt cannot make.
func TestBuild_ReturnTypeMatrix_EmitsCompilableGo(t *testing.T) {
	dir := t.TempDir()

	bp := `enum Status {
  active
  archived
}

entity Order {
  id: uuid @pk
  name: string
}

entity Ticket {
  id: uuid @pk
  opened_at: timestamp
}

entity LineItem {
  id: uuid @pk
  sku: string
}

entity Batch {
  id: uuid @pk
  lines: []LineItem
}

value Money {
  amount: decimal
  currency: string
}

value Stamp {
  at: timestamp
}

value Window {
  times: []timestamp
}

error OrderNotFound {
  kind: not_found
  message: "order not found"
}

action GetOrder(id: uuid) -> Order {
  let order = query one Order where id == id else OrderNotFound
  return order
}

action ListOrders(cursor: string, direction: string) -> []Order {
  let orders = query many Order order_by id asc limit 20 cursor cursor
  return orders
}

action StageOrders(id: uuid) -> []Order {
  return [Order { id: id, name: "staged" }]
}

action GetTicket(id: uuid) -> Ticket {
  let ticket = query one Ticket where id == id else OrderNotFound
  return ticket
}

action TicketsSince(from: timestamp, to: timestamp) -> Ticket {
  let ticket = query one Ticket where opened_at >= from and opened_at <= to else OrderNotFound
  return ticket
}

action StageBatch() -> Batch {
  let batch = create Batch {
    lines: [LineItem { id: "00000000-0000-0000-0000-000000000002", sku: "sku-1" }]
  }
  return batch
}

action EchoStatus(status: Status) -> Status {
  return status
}

action AddOne(n: int) -> int {
  return n + 1
}

action MakeMoney() -> Money {
  return Money { amount: 1.0, currency: "USD" }
}

action ListMoney() -> []Money {
  return [Money { amount: 1.0, currency: "USD" }]
}

action EmptyMoney() -> []Money {
  return []
}

action EchoStamp(stamp: Stamp) -> Stamp {
  return stamp
}

action EchoWindow(window: Window) -> Window {
  return window
}
`
	if err := os.WriteFile(filepath.Join(dir, "catalog.fdn"), []byte(bp), 0644); err != nil {
		t.Fatalf("write blueprint: %v", err)
	}

	buildProfile := `project:
  name: catalog
  module: example.com/catalog

storage:
  dialect: memory
  mode: in_memory

targets:
  - name: backend
    kind: backend
    language: go
    architecture: hexagonal
    http: chi
    out_dir: gen
`
	if err := os.WriteFile(filepath.Join(dir, "foundation.build.yaml"), []byte(buildProfile), 0644); err != nil {
		t.Fatalf("write foundation.build.yaml: %v", err)
	}

	if _, diags := compiler.Build(dir, compiler.BuildOptions{}); compiler.HasErrors(diags) {
		t.Fatalf("compiler.Build failed: %+v", diags)
	}

	gen := filepath.Join(dir, "gen")
	read := func(rel string) string {
		return readEmittedFile(t, filepath.Join(gen, filepath.FromSlash(rel)))
	}

	cases := []struct {
		name  string
		file  string
		want  string
		avoid string
	}{
		{"entity single (query one)", "internal/usecases/getorder_usecase.go", `var zero domain\.Order\b`, `var zero \*domain\.Order\b`},
		{"entity list (query many)", "internal/usecases/listorders_usecase.go", `Items: orders,`, `Items: \[\]domain\.Order`},
		{"entity list (query many)", "internal/domain/values.go", `Items\s+\[\]\*Order\s`, `Items\s+\[\]Order\s`},
		{"entity list (query many)", "internal/ports/order_repository.go", `\(\[\]\*domain\.Order, \*domain\.PageInfo, error\)`, `\(\[\]domain\.Order,`},
		{"entity list (literal)", "internal/usecases/stageorders_usecase.go", `Items: \[\]\*domain\.Order\{`, `Items: \[\]domain\.Order\{`},
		{"entity list (field and literal)", "internal/domain/batch_entity.go", `Lines\s+\[\]\*LineItem\s`, `Lines\s+\[\]LineItem\s`},
		{"entity list (field and literal)", "internal/usecases/stagebatch_usecase.go", `Lines: \[\]\*domain\.LineItem\{`, `Lines: \[\]domain\.LineItem\{`},
		{"entity with timestamp field", "internal/domain/ticket_entity.go", `OpenedAt\s+time\.Time`, `OpenedAt\s+string`},
		{"entity with timestamp field", "internal/domain/ticket_entity.go", `"time"`, ``},
		{"enum", "internal/usecases/echostatus_usecase.go", `status domain\.Status`, `status string`},
		{"enum", "internal/domain/enums.go", `StatusActive\s+Status = "active"`, ``},
		{"scalar", "internal/usecases/addone_usecase.go", `var finalRes int\b`, `var finalRes \*int`},
		{"value single", "internal/usecases/makemoney_usecase.go", `return domain\.Money\{`, `return \*domain\.Money\{`},
		{"value list", "internal/usecases/listmoney_usecase.go", `Items: \[\]domain\.Money\{`, `Items: \[\]\*domain\.Money\{`},
		{"value list", "internal/domain/values.go", `Items\s+\[\]Money\s`, `Items\s+\[\]\*Money\s`},
		{"empty value list", "internal/usecases/emptymoney_usecase.go", `Items: (nil|\[\]domain\.Money\{\}),`, ``},
		{"value with timestamp field", "internal/domain/values.go", `At\s+time\.Time`, ``},
		{"value with timestamp field", "internal/domain/values.go", `"time"`, ``},
		{"timestamp list", "internal/domain/values.go", `Times\s+\[\]time\.Time`, ``},
		{"timestamp list", "internal/usecases/echowindow_usecase.go", `window domain\.Window`, `time\.`},
		{"timestamp params (ports)", "internal/ports/ticket_repository.go", `opened_at_gte time\.Time, opened_at_lte time\.Time`, ``},
		{"timestamp params (ports)", "internal/ports/ticket_repository.go", `"time"`, ``},
		{"timestamp params (router)", "internal/adapters/http/router.go", `From\s+time\.Time`, ``},
		{"timestamp params (router)", "internal/adapters/http/router.go", `"time"`, ``},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			src := read(c.file)
			want := regexp.MustCompile(c.want)
			if !want.MatchString(src) {
				t.Errorf("%s: expected %s to match %q, got:\n%s", c.name, c.file, c.want, src)
			}
			if c.avoid != "" {
				if avoid := regexp.MustCompile(c.avoid); avoid.MatchString(src) {
					t.Errorf("%s: expected %s not to match %q, got:\n%s", c.name, c.file, c.avoid, src)
				}
			}
		})
	}

	t.Run("no malformed composite literals", func(t *testing.T) {
		malformed := regexp.MustCompile(`\[\]\*?[A-Za-z0-9_]*\.\{\}`)
		err := filepath.WalkDir(gen, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
				return err
			}
			if bad := malformed.FindString(readEmittedFile(t, path)); bad != "" {
				t.Errorf("emitted %s renders the malformed composite literal %q", path, bad)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk emitted module: %v", err)
		}
	})

	t.Run("emitted module compiles", func(t *testing.T) {
		if _, err := exec.LookPath("go"); err != nil {
			t.Fatalf("the emitted module cannot be compiled because the go toolchain is unavailable: %v", err)
		}
		cmd := exec.Command("go", "build", "./...")
		cmd.Dir = gen
		cmd.Env = append(os.Environ(), "GOPROXY=off", "GOWORK=off")
		out, err := cmd.CombinedOutput()
		if err == nil {
			return
		}
		if isGoModuleResolutionError(string(out)) {
			t.Fatalf("go build could not resolve the emitted module's dependencies offline, so this matrix did NOT verify that the emitted Go compiles:\n%v\n%s", err, out)
		}
		t.Fatalf("emitted module failed to compile: %v\n%s", err, out)
	})
}
