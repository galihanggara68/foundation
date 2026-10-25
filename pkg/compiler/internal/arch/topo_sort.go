package arch

import (
	"sort"
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

// SortEntitiesByDependency orders entities so that referenced entities appear before
// the entities that reference them via @fk traits. Ties are broken deterministically by entity name.
func SortEntitiesByDependency(entities []ir.EntityDecl) []ir.EntityDecl {
	byName := make(map[string]ir.EntityDecl, len(entities))
	for _, e := range entities {
		byName[e.Name] = e
	}

	// Calculate direct dependencies (entities this entity depends on)
	deps := make(map[string][]string, len(entities))
	dependents := make(map[string][]string, len(entities))
	inDegree := make(map[string]int, len(entities))

	for _, e := range entities {
		inDegree[e.Name] = 0
	}

	for _, e := range entities {
		seen := make(map[string]bool)
		for _, f := range e.Fields {
			fkRef, ok := getFieldTraitArg(f, "fk")
			if !ok {
				continue
			}
			parts := strings.SplitN(fkRef, ".", 2)
			targetName := parts[0]
			if targetName == e.Name || seen[targetName] {
				continue
			}
			if _, exists := byName[targetName]; exists {
				seen[targetName] = true
				deps[e.Name] = append(deps[e.Name], targetName)
				dependents[targetName] = append(dependents[targetName], e.Name)
				inDegree[e.Name]++
			}
		}
	}

	// Kahn's algorithm with deterministic tie-breaking
	var ready []string
	for _, e := range entities {
		if inDegree[e.Name] == 0 {
			ready = append(ready, e.Name)
		}
	}
	sort.Strings(ready)

	var ordered []ir.EntityDecl
	for len(ready) > 0 {
		curr := ready[0]
		ready = ready[1:]
		ordered = append(ordered, byName[curr])

		var newlyReady []string
		for _, dep := range dependents[curr] {
			inDegree[dep]--
			if inDegree[dep] == 0 {
				newlyReady = append(newlyReady, dep)
			}
		}
		if len(newlyReady) > 0 {
			sort.Strings(newlyReady)
			ready = append(ready, newlyReady...)
			sort.Strings(ready)
		}
	}

	// If there's an unresolved cycle, append remaining entities in original order
	if len(ordered) < len(entities) {
		visited := make(map[string]bool, len(ordered))
		for _, o := range ordered {
			visited[o.Name] = true
		}
		for _, e := range entities {
			if !visited[e.Name] {
				ordered = append(ordered, e)
			}
		}
	}

	return ordered
}
