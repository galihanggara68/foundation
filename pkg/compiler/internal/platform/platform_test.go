package platform_test

import (
	"testing"

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
