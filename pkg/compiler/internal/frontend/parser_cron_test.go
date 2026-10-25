package frontend_test

import (
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/frontend"
)

func TestParseFile_CronTrait_ReturnsFDN0115(t *testing.T) {
	src := `
@cron("0 * * * *")
action RunDailyReport() -> int {
  return 1
}
`
	_, diags := frontend.ParseFile("test.fdn", src)
	if len(diags) == 0 {
		t.Fatalf("expected diagnostic for @cron, got 0")
	}
	found := false
	for _, d := range diags {
		if d.Code == "FDN0115" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected diagnostic code FDN0115, got: %+v", diags)
	}
}
