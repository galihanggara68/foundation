package platform_test

import (
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/platform"
)

func TestTraitNamespace(t *testing.T) {
	tests := []struct {
		input     string
		wantNs    string
		wantLocal string
		wantIsNs  bool
	}{
		{input: "table", wantNs: "", wantLocal: "table", wantIsNs: false},
		{input: "acme:cached", wantNs: "acme", wantLocal: "cached", wantIsNs: true},
		{input: "foundation:custom", wantNs: "foundation", wantLocal: "custom", wantIsNs: true},
		{input: "acme:", wantNs: "acme", wantLocal: "", wantIsNs: true},
		{input: ":local", wantNs: "", wantLocal: "local", wantIsNs: false},
	}

	for _, tt := range tests {
		gotNs, gotLocal := platform.TraitNamespace(tt.input)
		if gotNs != tt.wantNs || gotLocal != tt.wantLocal {
			t.Errorf("TraitNamespace(%q) = (%q, %q), want (%q, %q)", tt.input, gotNs, gotLocal, tt.wantNs, tt.wantLocal)
		}
		if got := platform.IsNamespacedTrait(tt.input); got != tt.wantIsNs {
			t.Errorf("IsNamespacedTrait(%q) = %v, want %v", tt.input, got, tt.wantIsNs)
		}
	}
}

func TestStandardTraits(t *testing.T) {
	traits := platform.StandardTraits()
	if len(traits) == 0 {
		t.Fatalf("expected non-empty standard traits")
	}

	// Verify lookup
	for _, tr := range traits {
		if !platform.IsStandardTrait(tr.Name) {
			t.Errorf("expected IsStandardTrait(%q) to be true", tr.Name)
		}
		got, ok := platform.StandardTrait(tr.Name)
		if !ok || got.Name != tr.Name {
			t.Errorf("StandardTrait(%q) failed or returned mismatched trait: %+v", tr.Name, got)
		}
	}

	if platform.IsStandardTrait("non_existent_trait") {
		t.Errorf("IsStandardTrait('non_existent_trait') should be false")
	}
}

func TestTrait_Arg(t *testing.T) {
	tr, ok := platform.StandardTrait("event_handler")
	if !ok {
		t.Fatalf("expected 'event_handler' trait")
	}

	// Primary argument name
	arg, ok := tr.Arg("event")
	if !ok || arg.Name != "event" || !arg.Required {
		t.Errorf("expected required arg 'event', got %+v (ok=%v)", arg, ok)
	}

	// Positional alias "_"
	aliasArg, ok := tr.Arg("_")
	if !ok || aliasArg.Name != "event" {
		t.Errorf("expected alias '_' to resolve to 'event', got %+v (ok=%v)", aliasArg, ok)
	}

	// Non-existent arg
	if _, ok := tr.Arg("unknown"); ok {
		t.Errorf("expected Arg('unknown') to return false")
	}

	// FK trait argument contract
	fkTrait, ok := platform.StandardTrait("fk")
	if !ok {
		t.Fatalf("expected 'fk' trait")
	}
	if fkArg, ok := fkTrait.Arg("target"); !ok || !fkArg.Required {
		t.Errorf("expected required 'target' arg on @fk, got %+v", fkArg)
	}
	if _, ok := fkTrait.Arg("_"); !ok {
		t.Errorf("expected positional '_' alias on @fk")
	}

	// Renamed_from trait argument contract
	rfTrait, ok := platform.StandardTrait("renamed_from")
	if !ok {
		t.Fatalf("expected 'renamed_from' trait")
	}
	if rfArg, ok := rfTrait.Arg("name"); !ok || !rfArg.Required {
		t.Errorf("expected required 'name' arg on @renamed_from, got %+v", rfArg)
	}
	if _, ok := rfTrait.Arg("_"); !ok {
		t.Errorf("expected positional '_' alias on @renamed_from")
	}

	// Index trait hosts and argument contract
	idxTrait, ok := platform.StandardTrait("index")
	if !ok {
		t.Fatalf("expected 'index' trait")
	}
	if len(idxTrait.Hosts) != 2 || idxTrait.Hosts[0] != platform.TraitOnField || idxTrait.Hosts[1] != platform.TraitOnEntity {
		t.Errorf("expected @index to support field and entity hosts, got %+v", idxTrait.Hosts)
	}
	if fieldsArg, ok := idxTrait.Arg("fields"); !ok {
		t.Errorf("expected 'fields' arg on @index, got none")
	} else if len(fieldsArg.Aliases) == 0 || fieldsArg.Aliases[0] != "_" {
		t.Errorf("expected positional '_' alias on @index 'fields' arg, got %+v", fieldsArg.Aliases)
	}
	if _, ok := idxTrait.Arg("name"); !ok {
		t.Errorf("expected optional 'name' arg on @index")
	}

	// Unique trait hosts and argument contract
	uniqTrait, ok := platform.StandardTrait("unique")
	if !ok {
		t.Fatalf("expected 'unique' trait")
	}
	if len(uniqTrait.Hosts) != 2 || uniqTrait.Hosts[0] != platform.TraitOnField || uniqTrait.Hosts[1] != platform.TraitOnEntity {
		t.Errorf("expected @unique to support field and entity hosts, got %+v", uniqTrait.Hosts)
	}
	if fieldsArg, ok := uniqTrait.Arg("fields"); !ok {
		t.Errorf("expected 'fields' arg on @unique, got none")
	} else if len(fieldsArg.Aliases) == 0 || fieldsArg.Aliases[0] != "_" {
		t.Errorf("expected positional '_' alias on @unique 'fields' arg, got %+v", fieldsArg.Aliases)
	}
	if _, ok := uniqTrait.Arg("name"); !ok {
		t.Errorf("expected @unique to accept optional 'name' argument")
	}

	// Standard traits @len and @range
	lenTrait, ok := platform.StandardTrait("len")
	if !ok {
		t.Fatalf("expected StandardTrait('len') to succeed")
	}
	if len(lenTrait.Args) != 2 || lenTrait.Args[0].Name != "min" || lenTrait.Args[1].Name != "max" {
		t.Errorf("expected @len args min, max, got %+v", lenTrait.Args)
	}

	rangeTrait, ok := platform.StandardTrait("range")
	if !ok {
		t.Fatalf("expected StandardTrait('range') to succeed")
	}
	if len(rangeTrait.Args) != 2 || rangeTrait.Args[0].Name != "min" || rangeTrait.Args[1].Name != "max" {
		t.Errorf("expected @range args min, max, got %+v", rangeTrait.Args)
	}
}
