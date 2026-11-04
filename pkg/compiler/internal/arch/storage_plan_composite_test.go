package arch_test

import (
	"strings"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
)

func TestCompositeTrait_UniqueConstraint_CanonicalName(t *testing.T) {
	ent := ir.EntityDecl{
		Name: "Member",
		Fields: []ir.FieldDecl{
			{Name: "id", Type: "uuid", Traits: []ir.TraitDecl{{Name: "pk"}}},
			{Name: "organization_id", Type: "uuid"},
			{Name: "email", Type: "string"},
		},
		Traits: []ir.TraitDecl{
			{
				Name:     "unique",
				ListArgs: map[string][]string{"fields": {"organization_id", "email"}},
			},
		},
	}

	sem := &ir.SemanticIR{
		Entities: []ir.EntityDecl{ent},
	}
	bp := &arch.BuildProfile{
		Storage: arch.StorageConfig{
			Dialect: "postgres",
			Mode:    "declarative",
		},
	}

	plan, _, diags := arch.BuildStoragePlan(t.TempDir(), sem, bp, false)
	if len(diags) > 0 {
		t.Fatalf("unexpected diagnostics: %+v", diags)
	}

	expectedConstraint := "CONSTRAINT uq_members_organization_id_email UNIQUE (organization_id, email)"
	if !strings.Contains(plan.SchemaSQL, expectedConstraint) {
		t.Errorf("expected schema SQL to contain %q, got:\n%s", expectedConstraint, plan.SchemaSQL)
	}

	expectedTable := "CREATE TABLE members (\n" +
		"  id UUID PRIMARY KEY,\n" +
		"  organization_id UUID NOT NULL,\n" +
		"  email TEXT NOT NULL,\n" +
		"  CONSTRAINT uq_members_organization_id_email UNIQUE (organization_id, email)\n" +
		");"
	if !strings.Contains(plan.SchemaSQL, expectedTable) {
		t.Errorf("expected schema SQL to contain table definition:\n%s\ngot:\n%s", expectedTable, plan.SchemaSQL)
	}
}

func TestCompositeTrait_UniqueConstraint_CustomName(t *testing.T) {
	ent := ir.EntityDecl{
		Name: "Member",
		Fields: []ir.FieldDecl{
			{Name: "id", Type: "uuid", Traits: []ir.TraitDecl{{Name: "pk"}}},
			{Name: "tenant_id", Type: "uuid"},
			{Name: "user_id", Type: "uuid"},
		},
		Traits: []ir.TraitDecl{
			{
				Name:     "unique",
				Args:     map[string]string{"name": "uq_custom_member"},
				ListArgs: map[string][]string{"fields": {"tenant_id", "user_id"}},
			},
		},
	}

	constraints := arch.CompositeUniqueConstraintsSQL(ent, "members")
	if len(constraints) != 1 {
		t.Fatalf("expected 1 constraint, got %d", len(constraints))
	}

	expected := "CONSTRAINT uq_custom_member UNIQUE (tenant_id, user_id)"
	if constraints[0] != expected {
		t.Errorf("expected %q, got %q", expected, constraints[0])
	}
}

func TestCompositeTrait_Index_CanonicalName(t *testing.T) {
	ent := ir.EntityDecl{
		Name: "Order",
		Fields: []ir.FieldDecl{
			{Name: "id", Type: "uuid", Traits: []ir.TraitDecl{{Name: "pk"}}},
			{Name: "customer_id", Type: "uuid"},
			{Name: "status", Type: "string"},
		},
		Traits: []ir.TraitDecl{
			{
				Name:     "index",
				ListArgs: map[string][]string{"fields": {"customer_id", "status"}},
			},
		},
	}

	sem := &ir.SemanticIR{
		Entities: []ir.EntityDecl{ent},
	}
	bp := &arch.BuildProfile{
		Storage: arch.StorageConfig{
			Dialect: "postgres",
			Mode:    "declarative",
		},
	}

	plan, _, diags := arch.BuildStoragePlan(t.TempDir(), sem, bp, false)
	if len(diags) > 0 {
		t.Fatalf("unexpected diagnostics: %+v", diags)
	}

	expectedIndex := "CREATE INDEX idx_orders_customer_id_status ON orders (customer_id, status);\n"
	if !strings.Contains(plan.SchemaSQL, expectedIndex) {
		t.Errorf("expected schema SQL to contain %q, got:\n%s", expectedIndex, plan.SchemaSQL)
	}
}

func TestCompositeTrait_Index_CustomName(t *testing.T) {
	ent := ir.EntityDecl{
		Name: "AuditLog",
		Fields: []ir.FieldDecl{
			{Name: "id", Type: "uuid", Traits: []ir.TraitDecl{{Name: "pk"}}},
			{Name: "tenant_id", Type: "uuid"},
			{Name: "created_at", Type: "timestamp"},
		},
		Traits: []ir.TraitDecl{
			{
				Name:     "index",
				Args:     map[string]string{"name": "idx_custom_audit"},
				ListArgs: map[string][]string{"fields": {"tenant_id", "created_at"}},
			},
		},
	}

	sem := &ir.SemanticIR{
		Entities: []ir.EntityDecl{ent},
	}
	bp := &arch.BuildProfile{
		Storage: arch.StorageConfig{
			Dialect: "postgres",
			Mode:    "declarative",
		},
	}

	plan, _, diags := arch.BuildStoragePlan(t.TempDir(), sem, bp, false)
	if len(diags) > 0 {
		t.Fatalf("unexpected diagnostics: %+v", diags)
	}

	expectedIndex := "CREATE INDEX idx_custom_audit ON audit_logs (tenant_id, created_at);\n"
	if !strings.Contains(plan.SchemaSQL, expectedIndex) {
		t.Errorf("expected schema SQL to contain %q, got:\n%s", expectedIndex, plan.SchemaSQL)
	}
}

func TestCompositeTrait_MultipleUniqueAndIndexes(t *testing.T) {
	ent := ir.EntityDecl{
		Name: "Item",
		Fields: []ir.FieldDecl{
			{Name: "id", Type: "uuid", Traits: []ir.TraitDecl{{Name: "pk"}}},
			{Name: "col_a", Type: "string"},
			{Name: "col_b", Type: "string"},
			{Name: "col_c", Type: "string"},
		},
		Traits: []ir.TraitDecl{
			{
				Name:     "unique",
				ListArgs: map[string][]string{"fields": {"col_a", "col_b"}},
			},
			{
				Name:     "unique",
				Args:     map[string]string{"name": "uq_items_b_c"},
				ListArgs: map[string][]string{"fields": {"col_b", "col_c"}},
			},
			{
				Name:     "index",
				ListArgs: map[string][]string{"fields": {"col_a", "col_c"}},
			},
			{
				Name:     "index",
				Args:     map[string]string{"name": "idx_items_triplet"},
				ListArgs: map[string][]string{"fields": {"col_a", "col_b", "col_c"}},
			},
		},
	}

	sem := &ir.SemanticIR{
		Entities: []ir.EntityDecl{ent},
	}
	bp := &arch.BuildProfile{
		Storage: arch.StorageConfig{
			Dialect: "postgres",
			Mode:    "declarative",
		},
	}

	plan, _, diags := arch.BuildStoragePlan(t.TempDir(), sem, bp, false)
	if len(diags) > 0 {
		t.Fatalf("unexpected diagnostics: %+v", diags)
	}

	expectedTable := "CREATE TABLE items (\n" +
		"  id UUID PRIMARY KEY,\n" +
		"  col_a TEXT NOT NULL,\n" +
		"  col_b TEXT NOT NULL,\n" +
		"  col_c TEXT NOT NULL,\n" +
		"  CONSTRAINT uq_items_col_a_col_b UNIQUE (col_a, col_b),\n" +
		"  CONSTRAINT uq_items_b_c UNIQUE (col_b, col_c)\n" +
		");"
	if !strings.Contains(plan.SchemaSQL, expectedTable) {
		t.Errorf("expected schema SQL to contain table definition:\n%s\ngot:\n%s", expectedTable, plan.SchemaSQL)
	}

	expectedIdx1 := "CREATE INDEX idx_items_col_a_col_c ON items (col_a, col_c);\n"
	expectedIdx2 := "CREATE INDEX idx_items_triplet ON items (col_a, col_b, col_c);\n"
	if !strings.Contains(plan.SchemaSQL, expectedIdx1) {
		t.Errorf("expected schema SQL to contain %q, got:\n%s", expectedIdx1, plan.SchemaSQL)
	}
	if !strings.Contains(plan.SchemaSQL, expectedIdx2) {
		t.Errorf("expected schema SQL to contain %q, got:\n%s", expectedIdx2, plan.SchemaSQL)
	}
}

func TestCompositeTrait_FieldAndCompositeCombinations(t *testing.T) {
	ent := ir.EntityDecl{
		Name: "Account",
		Fields: []ir.FieldDecl{
			{Name: "id", Type: "uuid", Traits: []ir.TraitDecl{{Name: "pk"}}},
			{Name: "email", Type: "string", Traits: []ir.TraitDecl{{Name: "unique"}}},
			{Name: "status", Type: "string", Traits: []ir.TraitDecl{{Name: "index"}}},
			{Name: "country", Type: "string"},
		},
		Traits: []ir.TraitDecl{
			{
				Name:     "unique",
				ListArgs: map[string][]string{"fields": {"email", "country"}},
			},
			{
				Name:     "index",
				ListArgs: map[string][]string{"fields": {"country", "status"}},
			},
		},
	}

	sem := &ir.SemanticIR{
		Entities: []ir.EntityDecl{ent},
	}
	bp := &arch.BuildProfile{
		Storage: arch.StorageConfig{
			Dialect: "postgres",
			Mode:    "declarative",
		},
	}

	plan, _, diags := arch.BuildStoragePlan(t.TempDir(), sem, bp, false)
	if len(diags) > 0 {
		t.Fatalf("unexpected diagnostics: %+v", diags)
	}

	// Field-level unique is in column definition
	expectedEmailCol := "email TEXT NOT NULL UNIQUE"
	if !strings.Contains(plan.SchemaSQL, expectedEmailCol) {
		t.Errorf("expected schema SQL to contain %q", expectedEmailCol)
	}

	// Composite unique constraint
	expectedConstraint := "CONSTRAINT uq_accounts_email_country UNIQUE (email, country)"
	if !strings.Contains(plan.SchemaSQL, expectedConstraint) {
		t.Errorf("expected schema SQL to contain %q", expectedConstraint)
	}

	// Field-level index
	expectedFieldIdx := "CREATE INDEX idx_accounts_status ON accounts (status);\n"
	if !strings.Contains(plan.SchemaSQL, expectedFieldIdx) {
		t.Errorf("expected schema SQL to contain %q", expectedFieldIdx)
	}

	// Composite index
	expectedCompIdx := "CREATE INDEX idx_accounts_country_status ON accounts (country, status);\n"
	if !strings.Contains(plan.SchemaSQL, expectedCompIdx) {
		t.Errorf("expected schema SQL to contain %q", expectedCompIdx)
	}
}

func TestCompositeTrait_PositionalUnderscoreListArgs(t *testing.T) {
	ent := ir.EntityDecl{
		Name: "Project",
		Fields: []ir.FieldDecl{
			{Name: "id", Type: "uuid", Traits: []ir.TraitDecl{{Name: "pk"}}},
			{Name: "owner_id", Type: "uuid"},
			{Name: "slug", Type: "string"},
		},
		Traits: []ir.TraitDecl{
			{
				Name:     "unique",
				ListArgs: map[string][]string{"_": {"owner_id", "slug"}},
			},
			{
				Name:     "index",
				ListArgs: map[string][]string{"_": {"owner_id", "slug"}},
			},
		},
	}

	constraints := arch.CompositeUniqueConstraintsSQL(ent, "projects")
	if len(constraints) != 1 {
		t.Fatalf("expected 1 constraint, got %d", len(constraints))
	}
	expectedConstraint := "CONSTRAINT uq_projects_owner_id_slug UNIQUE (owner_id, slug)"
	if constraints[0] != expectedConstraint {
		t.Errorf("expected %q, got %q", expectedConstraint, constraints[0])
	}

	indexes := arch.CompositeIndexesSQL(ent, "projects")
	if len(indexes) != 1 {
		t.Fatalf("expected 1 index, got %d", len(indexes))
	}
	expectedIndex := "CREATE INDEX idx_projects_owner_id_slug ON projects (owner_id, slug);\n"
	if indexes[0] != expectedIndex {
		t.Errorf("expected %q, got %q", expectedIndex, indexes[0])
	}
}

func TestBuildTableSchemas_PopulatesTableModels(t *testing.T) {
	ent := ir.EntityDecl{
		Name: "Project",
		Fields: []ir.FieldDecl{
			{Name: "id", Type: "uuid", Traits: []ir.TraitDecl{{Name: "pk"}}},
			{Name: "owner_id", Type: "uuid", Traits: []ir.TraitDecl{{Name: "fk", Args: map[string]string{"_": "User.id"}}}},
			{Name: "slug", Type: "string"},
			{Name: "status", Type: "string", Traits: []ir.TraitDecl{{Name: "index"}}},
		},
		Traits: []ir.TraitDecl{
			{
				Name:     "unique",
				ListArgs: map[string][]string{"fields": {"owner_id", "slug"}},
				Args:     map[string]string{"name": "uq_custom_owner_slug"},
			},
		},
	}

	tableMap := map[string]string{"User": "users"}
	tables := arch.BuildTableSchemas([]ir.EntityDecl{ent}, map[string]bool{}, tableMap)
	if len(tables) != 1 {
		t.Fatalf("expected 1 table schema, got %d", len(tables))
	}
	tbl := tables[0]
	if tbl.Name != "projects" {
		t.Errorf("expected table name 'projects', got %q", tbl.Name)
	}
	if len(tbl.Columns) != 4 {
		t.Errorf("expected 4 columns, got %d", len(tbl.Columns))
	}
	if len(tbl.PrimaryKeys) != 1 || tbl.PrimaryKeys[0] != "id" {
		t.Errorf("expected PK ['id'], got %+v", tbl.PrimaryKeys)
	}
	if len(tbl.ForeignKeys) != 1 || tbl.ForeignKeys[0].ForeignTable != "users" {
		t.Errorf("expected FK targeting users, got %+v", tbl.ForeignKeys)
	}
	if len(tbl.Indexes) != 1 || tbl.Indexes[0].Name != "idx_projects_status" {
		t.Errorf("expected index 'idx_projects_status', got %+v", tbl.Indexes)
	}
	if len(tbl.Uniques) != 1 || tbl.Uniques[0].Name != "uq_custom_owner_slug" {
		t.Errorf("expected unique constraint 'uq_custom_owner_slug', got %+v", tbl.Uniques)
	}
}
