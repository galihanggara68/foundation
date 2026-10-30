package gohex

import (
	"fmt"
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

func (lay goLayout) emitGoMemoryEventPublisher() string {
	return fmt.Sprintf(`package %s

import (
	"context"
	"sync"
)

type PublishedEvent struct {
	EventType string
	Payload   any
}

type InMemoryEventPublisher struct {
	mu       sync.RWMutex
	events   []PublishedEvent
	handlers map[string][]func(ctx context.Context, payload any) error
}

func NewInMemoryEventPublisher() *InMemoryEventPublisher {
	return &InMemoryEventPublisher{
		handlers: make(map[string][]func(ctx context.Context, payload any) error),
	}
}

func (p *InMemoryEventPublisher) Subscribe(eventType string, handler func(ctx context.Context, payload any) error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.handlers[eventType] = append(p.handlers[eventType], handler)
}

func (p *InMemoryEventPublisher) Publish(ctx context.Context, eventType string, payload any) error {
	p.mu.Lock()
	p.events = append(p.events, PublishedEvent{
		EventType: eventType,
		Payload:   payload,
	})
	handlers := append([]func(ctx context.Context, payload any) error(nil), p.handlers[eventType]...)
	p.mu.Unlock()

	for _, h := range handlers {
		_ = h(ctx, payload)
	}
	return nil
}

func (p *InMemoryEventPublisher) ByType(eventType string) []any {
	p.mu.RLock()
	defer p.mu.RUnlock()
	var out []any
	for _, ev := range p.events {
		if ev.EventType == eventType {
			out = append(out, ev.Payload)
		}
	}
	return out
}
`, lay.MemoryPkg)
}

func (lay goLayout) emitGoMemoryTransactionManager() string {
	return fmt.Sprintf(`package %s

import (
	"context"
	"sync"
)

type Snapshotter interface {
	snapshot() func()
}

type InMemoryTransactionManager struct {
	mu    sync.Mutex
	repos []Snapshotter
}

func NewInMemoryTransactionManager(repos ...Snapshotter) *InMemoryTransactionManager {
	return &InMemoryTransactionManager{repos: repos}
}

func (tm *InMemoryTransactionManager) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	tm.mu.Lock()
	var restores []func()
	for _, r := range tm.repos {
		restores = append(restores, r.snapshot())
	}
	tm.mu.Unlock()

	err := fn(ctx)
	if err != nil {
		tm.mu.Lock()
		for _, restore := range restores {
			restore()
		}
		tm.mu.Unlock()
		return err
	}
	return nil
}
`, lay.MemoryPkg)
}

func (lay goLayout) emitGoSingleMemoryRepo(archIR *arch.ArchitectureIR, p arch.RepositoryPortIR) string {
	var sb strings.Builder
	hasTime := false
	hasFindMany := false
	for _, m := range p.Methods {
		if m.Kind == arch.RepoMethodFindMany {
			hasFindMany = true
		}
		for _, wp := range m.WherePredicates {
			if wp.Type == "timestamp" {
				hasTime = true
			}
		}
		if m.WhereType == "timestamp" {
			hasTime = true
		}
	}
	timeImport := ""
	if hasTime {
		timeImport = "\t\"time\"\n"
	}
	findManyImports := ""
	if hasFindMany {
		findManyImports = "\t\"sort\"\n\t\"strings\"\n"
	}
	sb.WriteString(fmt.Sprintf(`package %s

import (
	"context"
%s	"fmt"
%s	"sync"
%s	"%s/%s"
)

`, lay.MemoryPkg, findManyImports, "", timeImport, archIR.Module, lay.DomainDir))

	if hasFindMany {
		sb.WriteString("var (\n\t_ = sort.SliceStable\n\t_ = strings.Join\n)\n\n")
	}

	repoStruct := "InMemory" + p.EntityName + "Repository"
	sb.WriteString(fmt.Sprintf("type %s struct {\n\tmu sync.RWMutex\n\tentities []%s.%s\n\tseq int\n}\n\n", repoStruct, lay.DomainPkg, p.EntityName))
	sb.WriteString(fmt.Sprintf("func New%s() *%s {\n\treturn &%s{}\n}\n\n", repoStruct, repoStruct, repoStruct))
	sb.WriteString(fmt.Sprintf("func (r *%s) Seed(entity %s.%s) {\n\tr.mu.Lock()\n\tdefer r.mu.Unlock()\n\tr.entities = append(r.entities, entity)\n}\n\n", repoStruct, lay.DomainPkg, p.EntityName))
	sb.WriteString(fmt.Sprintf("func (r *%s) snapshot() func() {\n\tr.mu.RLock()\n\tsaved := append([]%s.%s(nil), r.entities...)\n\tsavedSeq := r.seq\n\tr.mu.RUnlock()\n\treturn func() {\n\t\tr.mu.Lock()\n\t\tr.entities = saved\n\t\tr.seq = savedSeq\n\t\tr.mu.Unlock()\n\t}\n}\n\n", repoStruct, lay.DomainPkg, p.EntityName))

	ent := archIR.EntityByName(p.EntityName)
	pkField, _ := arch.EntityPKField(ent)
	hasID := false
	for _, f := range ent.Fields {
		if f.Name == "id" && f.Kind == ir.KindUUID {
			hasID = true
		}
	}

	for _, m := range p.Methods {
		switch m.Kind {
		case arch.RepoMethodCreate:
			sb.WriteString(fmt.Sprintf("func (r *%s) Create(_ context.Context, entity %s.%s) (%s.%s, error) {\n",
				repoStruct, lay.DomainPkg, p.EntityName, lay.DomainPkg, p.EntityName))
			sb.WriteString("\tr.mu.Lock()\n\tdefer r.mu.Unlock()\n\tr.seq++\n")
			if hasID {
				sb.WriteString("\tif entity.ID == \"\" {\n\t\tentity.ID = fmt.Sprintf(\"00000000-0000-0000-0000-%012d\", r.seq)\n\t}\n")
			} else {
				sb.WriteString("\t_ = fmt.Sprintf\n")
			}
			for _, f := range ent.Fields {
				if strings.HasPrefix(f.DefaultVal, ".") {
					variant := strings.TrimPrefix(f.DefaultVal, ".")
					fExport := arch.ExportIdent(f.Name)
					sb.WriteString(fmt.Sprintf("\tif entity.%s == \"\" {\n\t\tentity.%s = %s.%s%s\n\t}\n",
						fExport, fExport, lay.DomainPkg, f.Type, arch.ExportIdent(variant)))
				}
			}
			if _, vExport, hasV := arch.EntityVersionField(ent); hasV {
				sb.WriteString(fmt.Sprintf("\tif entity.%s == 0 {\n\t\tentity.%s = 1\n\t}\n", vExport, vExport))
			}
			sb.WriteString("\tr.entities = append(r.entities, entity)\n\treturn entity, nil\n}\n\n")

		case arch.RepoMethodUpdate:
			sb.WriteString(fmt.Sprintf("func (r *%s) Update(_ context.Context, entity %s.%s) error {\n", repoStruct, lay.DomainPkg, p.EntityName))
			sb.WriteString("\tr.mu.Lock()\n\tdefer r.mu.Unlock()\n")
			_, vExport, hasV := arch.EntityVersionField(ent)
			if hasID {
				sb.WriteString("\tfor i := range r.entities {\n\t\tif r.entities[i].ID == entity.ID {\n")
				if hasV {
					sb.WriteString(fmt.Sprintf("\t\t\tif r.entities[i].%s != entity.%s {\n\t\t\t\treturn %s.ErrConflict\n\t\t\t}\n", vExport, vExport, lay.DomainPkg))
					sb.WriteString(fmt.Sprintf("\t\t\tentity.%s++\n", vExport))
				}
				sb.WriteString("\t\t\tr.entities[i] = entity\n\t\t\treturn nil\n\t\t}\n\t}\n")
			} else if len(ent.Fields) > 0 {
				firstExport := arch.ExportIdent(ent.Fields[0].Name)
				sb.WriteString(fmt.Sprintf("\tfor i := range r.entities {\n\t\tif r.entities[i].%s == entity.%s {\n", firstExport, firstExport))
				if hasV {
					sb.WriteString(fmt.Sprintf("\t\t\tif r.entities[i].%s != entity.%s {\n\t\t\t\treturn %s.ErrConflict\n\t\t\t}\n", vExport, vExport, lay.DomainPkg))
					sb.WriteString(fmt.Sprintf("\t\t\tentity.%s++\n", vExport))
				}
				sb.WriteString("\t\t\tr.entities[i] = entity\n\t\t\treturn nil\n\t\t}\n\t}\n")
			}
			if hasV {
				sb.WriteString(fmt.Sprintf("\treturn %s.ErrConflict\n}\n\n", lay.DomainPkg))
			} else {
				sb.WriteString("\treturn nil\n}\n\n")
			}

		case arch.RepoMethodDelete:
			sb.WriteString(fmt.Sprintf("func (r *%s) %s(_ context.Context, %s) error {\n",
				repoStruct, m.Name, lay.formatRepoParams(m)))
			sb.WriteString("\tr.mu.Lock()\n\tdefer r.mu.Unlock()\n")
			sb.WriteString(fmt.Sprintf("\tvar kept []%s.%s\n", lay.DomainPkg, p.EntityName))
			sb.WriteString("\tfor _, item := range r.entities {\n")
			fieldCounts := make(map[string]int)
			for _, wp := range m.WherePredicates {
				fieldCounts[wp.Field]++
			}
			var checks []string
			if len(m.WherePredicates) > 0 {
				for _, wp := range m.WherePredicates {
					pName := wp.Field
					if fieldCounts[wp.Field] > 1 {
						pName += "_" + strings.ToLower(arch.OpSuffix(wp.Op))
					}
					checks = append(checks, lay.formatInMemoryCheck("item."+arch.ExportIdent(wp.Field), wp.Op, pName, wp.Type))
				}
			} else {
				checks = append(checks, fmt.Sprintf("item.%s == %s", arch.ExportIdent(m.WhereField), m.WhereField))
			}
			sb.WriteString(fmt.Sprintf("\t\tif !(%s) {\n\t\t\tkept = append(kept, item)\n\t\t}\n", strings.Join(checks, " && ")))
			sb.WriteString("\t}\n\tr.entities = kept\n\treturn nil\n}\n\n")

		case arch.RepoMethodFind:
			sb.WriteString(fmt.Sprintf("func (r *%s) %s(_ context.Context, %s) (*%s.%s, error) {\n",
				repoStruct, m.Name, lay.formatRepoParams(m), lay.DomainPkg, p.EntityName))
			sb.WriteString("\tr.mu.RLock()\n\tdefer r.mu.RUnlock()\n")
			sb.WriteString("\tfor _, item := range r.entities {\n")
			fieldCounts := make(map[string]int)
			for _, wp := range m.WherePredicates {
				fieldCounts[wp.Field]++
			}
			var checks []string
			if len(m.WherePredicates) > 0 {
				for _, wp := range m.WherePredicates {
					pName := wp.Field
					if fieldCounts[wp.Field] > 1 {
						pName += "_" + strings.ToLower(arch.OpSuffix(wp.Op))
					}
					checks = append(checks, lay.formatInMemoryCheck("item."+arch.ExportIdent(wp.Field), wp.Op, pName, wp.Type))
				}
			} else {
				checks = append(checks, fmt.Sprintf("item.%s == %s", arch.ExportIdent(m.WhereField), m.WhereField))
			}
			sb.WriteString(fmt.Sprintf("\t\tif %s {\n\t\t\tcopyItem := item\n\t\t\treturn &copyItem, nil\n\t\t}\n", strings.Join(checks, " && ")))
			sb.WriteString("\t}\n\treturn nil, nil\n}\n\n")

		case arch.RepoMethodFindMany:
			whereParams := lay.formatRepoParams(m)
			callParams := "_ context.Context"
			if whereParams != "" {
				callParams += ", " + whereParams
			}
			callParams += ", cursor string, limit int, direction string"

			orderCol := m.OrderByField
			if orderCol == "" {
				orderCol = pkField
			}
			orderColExport := arch.ExportIdent(orderCol)
			orderDesc := m.OrderByDesc

			sb.WriteString(fmt.Sprintf("func (r *%s) %s(%s) ([]*%s.%s, *%s.PageInfo, error) {\n",
				repoStruct, m.Name, callParams, lay.DomainPkg, p.EntityName, lay.DomainPkg))
			sb.WriteString("\tr.mu.RLock()\n\tdefer r.mu.RUnlock()\n")
			sb.WriteString(fmt.Sprintf("\tvar filtered []%s.%s\n", lay.DomainPkg, p.EntityName))
			sb.WriteString("\tfor _, item := range r.entities {\n")

			var conds []string
			fieldCounts := make(map[string]int)
			for _, wp := range m.WherePredicates {
				fieldCounts[wp.Field]++
			}
			for _, wp := range m.WherePredicates {
				pName := wp.Field
				if fieldCounts[wp.Field] > 1 {
					pName += "_" + strings.ToLower(arch.OpSuffix(wp.Op))
				}
				conds = append(conds, lay.formatInMemoryCheck("item."+arch.ExportIdent(wp.Field), wp.Op, pName, wp.Type))
			}
			if len(conds) > 0 {
				sb.WriteString(fmt.Sprintf("\t\tif %s {\n\t\t\tfiltered = append(filtered, item)\n\t\t}\n", strings.Join(conds, " && ")))
			} else {
				sb.WriteString("\t\tfiltered = append(filtered, item)\n")
			}
			sb.WriteString("\t}\n")

			sb.WriteString("\tsort.SliceStable(filtered, func(i, j int) bool {\n")
			switch m.OrderByType {
			case "int":
				if orderDesc {
					sb.WriteString(fmt.Sprintf("\t\treturn filtered[i].%s > filtered[j].%s\n", orderColExport, orderColExport))
				} else {
					sb.WriteString(fmt.Sprintf("\t\treturn filtered[i].%s < filtered[j].%s\n", orderColExport, orderColExport))
				}
			case "timestamp":
				if orderDesc {
					sb.WriteString(fmt.Sprintf("\t\treturn filtered[i].%s.After(filtered[j].%s)\n", orderColExport, orderColExport))
				} else {
					sb.WriteString(fmt.Sprintf("\t\treturn filtered[i].%s.Before(filtered[j].%s)\n", orderColExport, orderColExport))
				}
			case "decimal":
				if orderDesc {
					sb.WriteString(fmt.Sprintf("\t\treturn filtered[i].%s.GreaterThan(filtered[j].%s)\n", orderColExport, orderColExport))
				} else {
					sb.WriteString(fmt.Sprintf("\t\treturn filtered[i].%s.LessThan(filtered[j].%s)\n", orderColExport, orderColExport))
				}
			default:
				if orderDesc {
					sb.WriteString(fmt.Sprintf("\t\treturn filtered[i].%s > filtered[j].%s\n", orderColExport, orderColExport))
				} else {
					sb.WriteString(fmt.Sprintf("\t\treturn filtered[i].%s < filtered[j].%s\n", orderColExport, orderColExport))
				}
			}
			sb.WriteString("\t})\n")

			sb.WriteString(fmt.Sprintf("\tcurVal := decodeCursor(cursor, %q)\n", orderCol))
			sb.WriteString(fmt.Sprintf("\tvar seeked []%s.%s\n", lay.DomainPkg, p.EntityName))
			sb.WriteString("\tif curVal != nil && cursor != \"\" {\n")
			if m.OrderByType == "int" {
				sb.WriteString("\t\tvar targetVal int64\n")
				sb.WriteString("\t\tswitch v := curVal.(type) {\n\t\tcase int64:\n\t\t\ttargetVal = v\n\t\tcase int:\n\t\t\ttargetVal = int64(v)\n\t\tcase float64:\n\t\t\ttargetVal = int64(v)\n\t\t}\n")
				sb.WriteString("\t\tif direction == \"prev\" {\n")
				sb.WriteString("\t\t\tfor i := len(filtered) - 1; i >= 0; i-- {\n")
				sb.WriteString(fmt.Sprintf("\t\t\t\titemVal := int64(filtered[i].%s)\n", orderColExport))
				if orderDesc {
					sb.WriteString("\t\t\t\tif itemVal > targetVal {\n\t\t\t\t\tseeked = append(seeked, filtered[i])\n\t\t\t\t}\n")
				} else {
					sb.WriteString("\t\t\t\tif itemVal < targetVal {\n\t\t\t\t\tseeked = append(seeked, filtered[i])\n\t\t\t\t}\n")
				}
				sb.WriteString("\t\t\t}\n\t\t} else {\n")
				sb.WriteString("\t\t\tfor _, item := range filtered {\n")
				sb.WriteString(fmt.Sprintf("\t\t\t\titemVal := int64(item.%s)\n", orderColExport))
				if orderDesc {
					sb.WriteString("\t\t\t\tif itemVal < targetVal {\n\t\t\t\t\tseeked = append(seeked, item)\n\t\t\t\t}\n")
				} else {
					sb.WriteString("\t\t\t\tif itemVal > targetVal {\n\t\t\t\t\tseeked = append(seeked, item)\n\t\t\t\t}\n")
				}
				sb.WriteString("\t\t\t}\n\t\t}\n")
			} else {
				sb.WriteString("\t\ttargetVal := fmt.Sprintf(\"%v\", curVal)\n")
				sb.WriteString("\t\tif direction == \"prev\" {\n")
				sb.WriteString("\t\t\tfor i := len(filtered) - 1; i >= 0; i-- {\n")
				sb.WriteString(fmt.Sprintf("\t\t\t\titemVal := fmt.Sprintf(\"%%v\", filtered[i].%s)\n", orderColExport))
				if orderDesc {
					sb.WriteString("\t\t\t\tif itemVal > targetVal {\n\t\t\t\t\tseeked = append(seeked, filtered[i])\n\t\t\t\t}\n")
				} else {
					sb.WriteString("\t\t\t\tif itemVal < targetVal {\n\t\t\t\t\tseeked = append(seeked, filtered[i])\n\t\t\t\t}\n")
				}
				sb.WriteString("\t\t\t}\n\t\t} else {\n")
				sb.WriteString("\t\t\tfor _, item := range filtered {\n")
				sb.WriteString(fmt.Sprintf("\t\t\t\titemVal := fmt.Sprintf(\"%%v\", item.%s)\n", orderColExport))
				if orderDesc {
					sb.WriteString("\t\t\t\tif itemVal < targetVal {\n\t\t\t\t\tseeked = append(seeked, item)\n\t\t\t\t}\n")
				} else {
					sb.WriteString("\t\t\t\tif itemVal > targetVal {\n\t\t\t\t\tseeked = append(seeked, item)\n\t\t\t\t}\n")
				}
				sb.WriteString("\t\t\t}\n\t\t}\n")
			}
			sb.WriteString("\t} else {\n\t\tseeked = filtered\n\t}\n")

			sb.WriteString("\thasNext := false\n\thasPrev := false\n")
			sb.WriteString("\tif curVal != nil && cursor != \"\" && direction == \"prev\" {\n")
			sb.WriteString("\t\thasPrev = len(seeked) > limit\n\t\tif hasPrev {\n\t\t\tseeked = seeked[:limit]\n\t\t}\n")
			sb.WriteString("\t\thasNext = true\n")
			sb.WriteString("\t\tfor i, j := 0, len(seeked)-1; i < j; i, j = i+1, j-1 {\n\t\t\tseeked[i], seeked[j] = seeked[j], seeked[i]\n\t\t}\n")
			sb.WriteString("\t} else {\n")
			sb.WriteString("\t\thasNext = len(seeked) > limit\n\t\tif hasNext {\n\t\t\tseeked = seeked[:limit]\n\t\t}\n")
			sb.WriteString("\t\tif cursor != \"\" {\n\t\t\thasPrev = true\n\t\t}\n\t}\n")

			sb.WriteString(fmt.Sprintf("\tvar items []*%s.%s\n", lay.DomainPkg, p.EntityName))
			sb.WriteString("\tfor i := range seeked {\n\t\tcopied := seeked[i]\n\t\titems = append(items, &copied)\n\t}\n")

			sb.WriteString("\tnextCursor := \"\"\n\tprevCursor := \"\"\n")
			sb.WriteString("\tif len(items) > 0 {\n")
			sb.WriteString(fmt.Sprintf("\t\tif hasNext {\n\t\t\tnextCursor = encodeCursor(items[len(items)-1].%s, %q)\n\t\t}\n", orderColExport, orderCol))
			sb.WriteString(fmt.Sprintf("\t\tif hasPrev {\n\t\t\tprevCursor = encodeCursor(items[0].%s, %q)\n\t\t}\n", orderColExport, orderCol))
			sb.WriteString("\t}\n")

			sb.WriteString(fmt.Sprintf("\treturn items, &%s.PageInfo{\n\t\tNextCursor: nextCursor,\n\t\tPrevCursor: prevCursor,\n\t\tHasNext: hasNext,\n\t\tHasPrev: hasPrev,\n\t}, nil\n}\n\n", lay.DomainPkg))
		}
	}

	return sb.String()
}
