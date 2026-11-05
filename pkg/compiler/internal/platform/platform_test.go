package platform_test

import (
	"reflect"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/platform"
)

func TestDefaultInfo_CarriesCompilerOwnedPaginationDefaults(t *testing.T) {
	got := platform.DefaultInfo()

	if got.Pagination.DefaultLimit != platform.DefaultPageLimit {
		t.Errorf("default_limit: got %d, want %d", got.Pagination.DefaultLimit, platform.DefaultPageLimit)
	}
	if got.Pagination.MaxLimit != platform.DefaultMaxPageLimit {
		t.Errorf("max_limit: got %d, want %d", got.Pagination.MaxLimit, platform.DefaultMaxPageLimit)
	}
}

func TestDefaultInfo_CarriesDependencyBOM(t *testing.T) {
	want := []platform.DependencyVersion{
		{Language: "go", Package: "github.com/galihanggara68/foundation/contrib/go/runtime", Version: "v0.2.0"},
		{Language: "go", Package: "github.com/go-chi/chi/v5", Version: "v5.3.0"},
		{Language: "go", Package: "github.com/jackc/pgx/v5", Version: "v5.7.2"},
		{Language: "go", Package: "github.com/lib/pq", Version: "v1.10.9"},
		{Language: "go", Package: "go.opentelemetry.io/otel", Version: "v1.32.0"},
		{Language: "go", Package: "gorm.io/gorm", Version: "v1.25.12"},
	}
	if got := platform.DefaultInfo().Dependencies; !reflect.DeepEqual(got, want) {
		t.Fatalf("dependency BOM mismatch:\n  got:  %+v\n  want: %+v", got, want)
	}
}

func TestStandardDependencies_ContribRuntimeMatchesCompilerPin(t *testing.T) {
	for _, dep := range platform.StandardDependencies() {
		if dep.Package == "github.com/galihanggara68/foundation/contrib/go/runtime" {
			if dep.Version != "v"+arch.ContribRuntimeVersion {
				t.Fatalf("contrib runtime BOM %q does not match arch.ContribRuntimeVersion %q", dep.Version, arch.ContribRuntimeVersion)
			}
			return
		}
	}
	t.Fatal("expected the contrib runtime in the BOM")
}

func TestDefaultInfo_CarriesStandardReservedErrors(t *testing.T) {
	info := platform.DefaultInfo()

	if len(info.ReservedErrors) != 7 {
		t.Fatalf("expected 7 reserved errors, got %d", len(info.ReservedErrors))
	}

	want := []platform.ReservedError{
		{
			Name:    "NotFound",
			Kind:    ir.ErrorKindNotFound,
			Status:  404,
			Posture: ir.PostureBusiness,
		},
		{
			Name:    "Validation",
			Kind:    ir.ErrorKindValidation,
			Status:  400,
			Posture: ir.PostureBusiness,
		},
		{
			Name:    "Conflict",
			Kind:    ir.ErrorKindConflict,
			Status:  409,
			Posture: ir.PostureBusiness,
		},
		{
			Name:    "Unauthenticated",
			Kind:    ir.ErrorKindUnauthenticated,
			Status:  401,
			Posture: ir.PostureBusiness,
		},
		{
			Name:    "Unauthorized",
			Kind:    ir.ErrorKindUnauthorized,
			Status:  403,
			Posture: ir.PostureBusiness,
		},
		{
			Name:    "Forbidden",
			Kind:    ir.ErrorKindForbidden,
			Status:  403,
			Posture: ir.PostureBusiness,
		},
		{
			Name:    "Internal",
			Kind:    ir.ErrorKindInternal,
			Status:  500,
			Posture: ir.PostureTechnical,
		},
	}

	for i, err := range info.ReservedErrors {
		expected := want[i]
		if err.Name != expected.Name {
			t.Errorf("[%d] Name: got %q, want %q", i, err.Name, expected.Name)
		}
		if err.Kind != expected.Kind {
			t.Errorf("[%d] Kind: got %q, want %q", i, err.Kind, expected.Kind)
		}
		if err.Status != expected.Status {
			t.Errorf("[%d] Status: got %d, want %d", i, err.Status, expected.Status)
		}
		if err.Posture != expected.Posture {
			t.Errorf("[%d] Posture: got %q, want %q", i, err.Posture, expected.Posture)
		}
	}
}

func TestStandardReservedErrors_ReturnsNewSliceEachCall(t *testing.T) {
	first := platform.StandardReservedErrors()
	first[0].Name = "Mutated"

	second := platform.StandardReservedErrors()
	if second[0].Name != "NotFound" {
		t.Errorf("expected clean slice, got mutated name %q", second[0].Name)
	}
}
