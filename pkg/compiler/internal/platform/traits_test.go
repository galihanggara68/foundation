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
}
