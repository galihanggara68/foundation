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
