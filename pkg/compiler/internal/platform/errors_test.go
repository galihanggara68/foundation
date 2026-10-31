package platform_test

import (
	"reflect"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/platform"
)

// These are the statuses the emitters hard-coded before the vocabulary existed:
// gohex http_server.go, tsclean controllers.go, and docs/blueprints/declarations.md.
// Keeping them here means a change to a kind's standard status has to be deliberate.
func TestErrorVocabulary_MatchesTheStatusesEmittersUsedToHardCode(t *testing.T) {
	want := map[ir.ErrorKind]int{
		ir.ErrorKindNotFound:        404,
		ir.ErrorKindValidation:      400,
		ir.ErrorKindConflict:        409,
		ir.ErrorKindUnauthenticated: 401,
		ir.ErrorKindUnauthorized:    401,
		ir.ErrorKindForbidden:       403,
		ir.ErrorKindInternal:        500,
	}

	for kind, wantStatus := range want {
		policy, standard := platform.ErrorPolicyFor(kind)
		if !standard {
			t.Errorf("kind %q is missing from the standard vocabulary", kind)
			continue
		}
		if policy.Status != wantStatus {
			t.Errorf("status for kind %q: got %d, want %d", kind, policy.Status, wantStatus)
		}
	}
}

func TestErrorPolicyFor_KindOutsideVocabulary_ReportsNotStandard(t *testing.T) {
	if _, standard := platform.ErrorPolicyFor(ir.ErrorKind("bogus")); standard {
		t.Error("expected a kind outside the vocabulary to be reported as not standard")
	}
}

// Every kind owns one name, and the vocabulary is the only place that set is written
// down — a declaration reusing one of these names is flagged against this table.
func TestReservedErrorName_ResolvesEveryKindsOwnName(t *testing.T) {
	want := map[string]ir.ErrorKind{
		"NotFound":        ir.ErrorKindNotFound,
		"Validation":      ir.ErrorKindValidation,
		"Conflict":        ir.ErrorKindConflict,
		"Unauthenticated": ir.ErrorKindUnauthenticated,
		"Unauthorized":    ir.ErrorKindUnauthorized,
		"Forbidden":       ir.ErrorKindForbidden,
		"Internal":        ir.ErrorKindInternal,
	}

	for name, wantKind := range want {
		kind, reserved := platform.ReservedErrorName(name)
		if !reserved {
			t.Errorf("%q must be reserved: it is the vocabulary's own name for kind %q", name, wantKind)
			continue
		}
		if kind != wantKind {
			t.Errorf("reserved name %q belongs to kind %q, got %q", name, wantKind, kind)
		}
	}
}

func TestReservedErrorName_NameOutsideVocabulary_IsNotReserved(t *testing.T) {
	if _, reserved := platform.ReservedErrorName("ItemNotFound"); reserved {
		t.Error("a name the vocabulary does not claim must not be reserved")
	}
}

// The vocabulary is hand-written, so this is what keeps it well-formed: every kind has
// exactly one reserved name and no name is shared. Without it a new kind could ship
// without a reserved name and no test would notice.
func TestReservedErrorNames_AreOnePerKindAndUnique(t *testing.T) {
	kinds := platform.StandardErrorKinds()
	names := platform.ReservedErrorNames()
	if len(names) != len(kinds) {
		t.Fatalf("expected one reserved name per kind (%d kinds), got %d: %v", len(kinds), len(names), names)
	}

	covered := map[ir.ErrorKind]string{}
	for _, name := range names {
		if name == "" {
			t.Error("every kind must have a reserved name")
			continue
		}
		kind, reserved := platform.ReservedErrorName(name)
		if !reserved {
			t.Errorf("a reserved name must resolve back to its kind, but %q does not", name)
			continue
		}
		if prev, seen := covered[kind]; seen {
			t.Errorf("kinds must not share a reserved name: %q and %q both claim %q", prev, name, kind)
			continue
		}
		covered[kind] = name
	}

	for _, kind := range kinds {
		if _, ok := covered[kind]; !ok {
			t.Errorf("kind %q has no reserved name", kind)
		}
	}
}

func TestBuiltinErrors_AreThePreSeededPairStampedFromTheVocabulary(t *testing.T) {
	byName := map[string]ir.ErrorDecl{}
	for _, errDecl := range platform.BuiltinErrors() {
		byName[errDecl.Name] = errDecl
	}
	if len(byName) != 2 {
		t.Fatalf("expected the two errors every service ships with, got %d: %v", len(byName), byName)
	}

	want := map[string]struct {
		kind    ir.ErrorKind
		status  int
		posture ir.ErrorPosture
		message string
	}{
		"Conflict":        {ir.ErrorKindConflict, 409, ir.PostureBusiness, "optimistic concurrency conflict"},
		"Unauthenticated": {ir.ErrorKindUnauthenticated, 401, ir.PostureBusiness, "unauthenticated actor"},
	}

	for name, wantErr := range want {
		got, ok := byName[name]
		if !ok {
			t.Errorf("expected a %q built-in", name)
			continue
		}
		if got.Kind != wantErr.kind {
			t.Errorf("%s kind: got %q, want %q", name, got.Kind, wantErr.kind)
		}
		if got.Message != wantErr.message {
			t.Errorf("%s message: got %q, want %q", name, got.Message, wantErr.message)
		}
		if got.Status != wantErr.status {
			t.Errorf("%s status: got %d, want %d", name, got.Status, wantErr.status)
		}
		if got.Posture != wantErr.posture {
			t.Errorf("%s posture: got %q, want %q", name, got.Posture, wantErr.posture)
		}
	}

	stamped := platform.BuiltinErrors()
	stamped[0].Name = "mutated"
	if platform.BuiltinErrors()[0].Name == "mutated" {
		t.Error("callers must not be able to mutate the vocabulary through the returned slice")
	}
}

func TestIsBuiltinErrorName_DistinguishesShippedFromMerelyReserved(t *testing.T) {
	if !platform.IsBuiltinErrorName("Conflict") {
		t.Error("Conflict is one of the errors every service ships with")
	}
	if platform.IsBuiltinErrorName("NotFound") {
		t.Error("NotFound is reserved by the vocabulary but is not an error services ship with")
	}
}

// The order is part of the contract: anything enumerating the vocabulary to emit code
// has to produce the same bytes on every build.
func TestStandardErrorKinds_ReturnsTheVocabularyInStableOrder(t *testing.T) {
	want := []ir.ErrorKind{
		ir.ErrorKindNotFound,
		ir.ErrorKindValidation,
		ir.ErrorKindConflict,
		ir.ErrorKindUnauthenticated,
		ir.ErrorKindUnauthorized,
		ir.ErrorKindForbidden,
		ir.ErrorKindInternal,
	}

	got := platform.StandardErrorKinds()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("standard error kinds: got %v, want %v", got, want)
	}

	got[0] = ir.ErrorKind("mutated")
	if platform.StandardErrorKinds()[0] != ir.ErrorKindNotFound {
		t.Error("callers must not be able to mutate the vocabulary through the returned slice")
	}
}
