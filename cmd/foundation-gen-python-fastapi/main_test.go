package main

import (
	"strings"
	"testing"

	"github.com/galihanggara68/foundation/pkg/emitterkit"
)

func fastAPITestRequest(page *emitterkit.PaginationDefaults) emitterkit.GenerationRequest {
	req := emitterkit.GenerationRequest{
		SemanticIR: &emitterkit.SemanticIR{
			Entities: []emitterkit.EntityDecl{
				{Name: "Item", Fields: []emitterkit.FieldDecl{{Name: "id", Type: "string"}}},
			},
		},
	}
	if page != nil {
		req.Platform = &emitterkit.PlatformInfo{Pagination: *page}
	}
	return req
}

func TestGeneratePythonFastAPI_ConsumesPlatformPagination(t *testing.T) {
	resp, err := generatePythonFastAPI(fastAPITestRequest(&emitterkit.PaginationDefaults{DefaultLimit: 7, MaxLimit: 42}))
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	repo := resp.Files["repository.py"]
	if !strings.Contains(repo, "DEFAULT_PAGE_LIMIT = 7") {
		t.Errorf("expected the platform default limit to drive the repository, got:\n%s", repo)
	}
	if !strings.Contains(repo, "MAX_PAGE_LIMIT = 42") {
		t.Errorf("expected the platform maximum limit to drive the repository, got:\n%s", repo)
	}
	if !strings.Contains(repo, "def find_all(self, limit: Optional[int] = None)") {
		t.Errorf("expected a bounded find_all, got:\n%s", repo)
	}
}

func TestGeneratePythonFastAPI_WithoutPlatformKeepsUnboundedRead(t *testing.T) {
	resp, err := generatePythonFastAPI(fastAPITestRequest(nil))
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	repo := resp.Files["repository.py"]
	if strings.Contains(repo, "DEFAULT_PAGE_LIMIT") {
		t.Errorf("a request without a platform section must not gain a pagination policy, got:\n%s", repo)
	}
	if !strings.Contains(repo, "def find_all(self) -> List[Item]:") {
		t.Errorf("expected the legacy unbounded find_all, got:\n%s", repo)
	}
}
