package arch

import (
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

type TacticalTable string

const (
	TableIdempotencyKeys TacticalTable = "idempotency_keys"
	TableOutboxEvents    TacticalTable = "outbox_events"
)

func hasActionWithTrait(actions []ir.ActionDecl, traitName string) bool {
	for _, a := range actions {
		if a.HasTrait(traitName) {
			return true
		}
	}
	return false
}

// HasIdempotentAction reports whether any action in the semantic model declares @idempotent.
func HasIdempotentAction(actions []ir.ActionDecl) bool {
	return hasActionWithTrait(actions, "idempotent")
}

// HasOutboxAction reports whether any action in the semantic model declares @outbox.
func HasOutboxAction(actions []ir.ActionDecl) bool {
	return hasActionWithTrait(actions, "outbox")
}

// GenerateIdempotencyTableSQL returns the PostgreSQL DDL for the idempotency_keys table.
func GenerateIdempotencyTableSQL() string {
	return `CREATE TABLE idempotency_keys (
  key TEXT PRIMARY KEY,
  status TEXT NOT NULL,
  response_payload BYTEA,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  expires_at TIMESTAMPTZ NOT NULL
);
`
}

// GenerateOutboxTableSQL returns the PostgreSQL DDL for the outbox_events table.
func GenerateOutboxTableSQL() string {
	return `CREATE TABLE outbox_events (
  id TEXT PRIMARY KEY,
  topic TEXT NOT NULL,
  event_name TEXT NOT NULL,
  payload BYTEA NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  published BOOLEAN NOT NULL DEFAULT FALSE
);
CREATE INDEX idx_outbox_events_published ON outbox_events (published, created_at);
`
}

// GenerateDropTacticalTablesSQL returns DROP TABLE statements for tactical tables in reverse dependency order.
func GenerateDropTacticalTablesSQL(hasIdem, hasOutbox bool) string {
	var sb strings.Builder
	if hasOutbox {
		sb.WriteString("DROP TABLE IF EXISTS " + string(TableOutboxEvents) + " CASCADE;\n")
	}
	if hasIdem {
		sb.WriteString("DROP TABLE IF EXISTS " + string(TableIdempotencyKeys) + " CASCADE;\n")
	}
	return sb.String()
}

// ActiveTacticalTables returns a sorted slice of active tactical table names.
func ActiveTacticalTables(hasIdem, hasOutbox bool) []string {
	var tables []string
	if hasIdem {
		tables = append(tables, string(TableIdempotencyKeys))
	}
	if hasOutbox {
		tables = append(tables, string(TableOutboxEvents))
	}
	return tables
}

// DiffTacticalTablesSQL appends migration SQL for tactical tables added or dropped between snapshots.
func DiffTacticalTablesSQL(prevTables []string, hasIdem, hasOutbox bool, upSB, downSB *strings.Builder) {
	prevMap := make(map[string]bool)
	for _, t := range prevTables {
		prevMap[t] = true
	}

	if hasIdem && !prevMap[string(TableIdempotencyKeys)] {
		upSB.WriteString(GenerateIdempotencyTableSQL() + "\n")
		downSB.WriteString("DROP TABLE IF EXISTS " + string(TableIdempotencyKeys) + " CASCADE;\n")
	} else if !hasIdem && prevMap[string(TableIdempotencyKeys)] {
		upSB.WriteString("DROP TABLE IF EXISTS " + string(TableIdempotencyKeys) + " CASCADE;\n")
		downSB.WriteString(GenerateIdempotencyTableSQL() + "\n")
	}

	if hasOutbox && !prevMap[string(TableOutboxEvents)] {
		upSB.WriteString(GenerateOutboxTableSQL() + "\n")
		downSB.WriteString("DROP TABLE IF EXISTS " + string(TableOutboxEvents) + " CASCADE;\n")
	} else if !hasOutbox && prevMap[string(TableOutboxEvents)] {
		upSB.WriteString("DROP TABLE IF EXISTS " + string(TableOutboxEvents) + " CASCADE;\n")
		downSB.WriteString(GenerateOutboxTableSQL() + "\n")
	}
}
