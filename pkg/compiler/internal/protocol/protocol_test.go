package protocol_test

import (
	"encoding/json"
	"testing"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/ir"
	"github.com/galihanggara68/foundation/pkg/compiler/internal/protocol"
)

func TestProtocolVersion(t *testing.T) {
	if protocol.ProtocolVersion != 1 {
		t.Fatalf("expected ProtocolVersion to be 1, got %d", protocol.ProtocolVersion)
	}
}

func TestGenerationRequest_RoundTrip(t *testing.T) {
	req := protocol.GenerationRequest{
		ProtocolVersion: protocol.ProtocolVersion,
		WorkspaceRoot:   "/workspace",
		Target: protocol.TargetInfo{
			TargetConfig: arch.TargetConfig{
				Name:         "backend",
				Kind:         "backend",
				Language:     "go",
				Architecture: "hexagonal",
				HTTP:         "chi",
				OutDir:       "gen/go",
			},
			Options: map[string]string{
				"telemetry": "otel",
			},
		},
		SemanticIR: &ir.SemanticIR{
			Entities: []ir.EntityDecl{
				{
					Name: "Order",
					Fields: []ir.FieldDecl{
						{Name: "id", Type: "UUID", Span: ir.Span{File: "domain.fdn", Line: 5, Column: 2}},
						{Name: "total", Type: "Money", Span: ir.Span{File: "domain.fdn", Line: 6, Column: 2}},
					},
					Span: ir.Span{File: "domain.fdn", Line: 4, Column: 1},
				},
			},
		},
		ArchitectureIR: &arch.ArchitectureIR{
			TargetName: "backend",
			Kind:       "backend",
			Language:   "go",
			Pattern:    "hexagonal",
			HTTP:       "chi",
			Module:     "example.com/shop",
			OutDir:     "gen/go",
		},
		Files: map[string]string{
			"go.mod": "module example.com/shop\n\ngo 1.24\n",
		},
	}

	data, err := protocol.EncodeRequest(req)
	if err != nil {
		t.Fatalf("EncodeRequest failed: %v", err)
	}

	decoded, err := protocol.DecodeRequest(data)
	if err != nil {
		t.Fatalf("DecodeRequest failed: %v", err)
	}

	if decoded.ProtocolVersion != req.ProtocolVersion {
		t.Errorf("expected ProtocolVersion %d, got %d", req.ProtocolVersion, decoded.ProtocolVersion)
	}
	if decoded.WorkspaceRoot != req.WorkspaceRoot {
		t.Errorf("expected WorkspaceRoot %q, got %q", req.WorkspaceRoot, decoded.WorkspaceRoot)
	}
	if decoded.Target.Name != req.Target.Name || decoded.Target.Language != req.Target.Language {
		t.Errorf("Target mismatch: %+v vs %+v", decoded.Target, req.Target)
	}
	if decoded.Target.Options["telemetry"] != "otel" {
		t.Errorf("Target options mismatch: %+v", decoded.Target.Options)
	}
	if len(decoded.SemanticIR.Entities) != 1 || decoded.SemanticIR.Entities[0].Name != "Order" {
		t.Errorf("SemanticIR entities mismatch: %+v", decoded.SemanticIR)
	}
	if decoded.ArchitectureIR == nil || decoded.ArchitectureIR.Pattern != "hexagonal" {
		t.Errorf("ArchitectureIR mismatch: %+v", decoded.ArchitectureIR)
	}
	if decoded.Files["go.mod"] != req.Files["go.mod"] {
		t.Errorf("Files mismatch: got %v", decoded.Files)
	}
}

func TestGenerationRequest_FullPayloadRoundTrip(t *testing.T) {
	sem := &ir.SemanticIR{
		Actors: []ir.ActorDecl{
			{
				Name: "User",
				Fields: []ir.FieldDecl{
					{Name: "id", Type: "uuid"},
					{Name: "email", Type: "string"},
				},
				Span: ir.Span{File: "auth.fdn", Line: 1, Column: 1},
			},
		},
		Enums: []ir.EnumDecl{
			{Name: "Status", Variants: []string{"active", "inactive"}, Span: ir.Span{File: "types.fdn", Line: 1, Column: 1}},
		},
		Values: []ir.ValueDecl{
			{
				Name: "Money",
				Fields: []ir.FieldDecl{
					{Name: "amount", Type: "decimal"},
					{Name: "currency", Type: "string"},
				},
				Span: ir.Span{File: "types.fdn", Line: 5, Column: 1},
			},
		},
		Entities: []ir.EntityDecl{
			{
				Name: "Item",
				Fields: []ir.FieldDecl{
					{Name: "id", Type: "uuid", Traits: []ir.TraitDecl{{Name: "pk"}}},
					{Name: "name", Type: "string"},
				},
				Span: ir.Span{File: "shop.fdn", Line: 10, Column: 1},
			},
		},
		Errors: []ir.ErrorDecl{
			{Name: "ItemNotFound", Kind: "not_found", Message: "item not found", Span: ir.Span{File: "shop.fdn", Line: 20, Column: 1}},
		},
		Events: []ir.EventDecl{
			{
				Name: "ItemCreated",
				Fields: []ir.FieldDecl{
					{Name: "id", Type: "uuid"},
				},
				Span: ir.Span{File: "shop.fdn", Line: 25, Column: 1},
			},
		},
		Policies: []ir.PolicyDecl{
			{
				Name: "CanAccess",
				Params: []ir.ParamDecl{
					{Name: "u", Type: "User"},
				},
				Span: ir.Span{File: "shop.fdn", Line: 30, Column: 1},
			},
		},
		Slots: []ir.SlotDecl{
			{
				Name: "PaymentGateway",
				Methods: []ir.SlotMethodDecl{
					{
						Name:       "Charge",
						Params:     []ir.ParamDecl{{Name: "amount", Type: "Money"}},
						ReturnType: "string",
						Throws:     []string{"PaymentFailed"},
					},
				},
				Span: ir.Span{File: "shop.fdn", Line: 35, Column: 1},
			},
		},
		Actions: []ir.ActionDecl{
			{
				Name:       "CreateItem",
				ReturnType: "Item",
				Params:     []ir.ParamDecl{{Name: "name", Type: "string"}},
				Traits:     []ir.TraitDecl{{Name: "http", Args: map[string]string{"method": "POST", "path": "/items"}}},
				Span:       ir.Span{File: "shop.fdn", Line: 45, Column: 1},
			},
		},
		Scenarios: []ir.ScenarioDecl{
			{
				Name:       "CreateValidItem",
				WhenAction: "CreateItem",
				WhenArgs:   []ir.Expr{{Kind: ir.ExprString, Value: "Book"}},
				Expects: []ir.ExpectDecl{
					{
						Left:  ir.Expr{Kind: ir.ExprIdent, Value: "res"},
						Op:    "==",
						Right: ir.Expr{Kind: ir.ExprIdent, Value: "expected"},
					},
				},
				Span: ir.Span{File: "shop.fdn", Line: 55, Column: 1},
			},
		},
	}

	bp := &arch.BuildProfile{
		Project: arch.ProjectConfig{Name: "shop", Module: "example.com/shop"},
		Storage: arch.StorageConfig{Dialect: "postgres", Mode: "hybrid"},
		Targets: []arch.TargetConfig{
			{Name: "backend", Kind: "backend", Language: "go", Architecture: "hexagonal", HTTP: "chi", OutDir: "gen/go"},
		},
	}

	archIR, diags := arch.Lower(sem, bp, bp.Targets[0])
	if len(diags) > 0 {
		t.Fatalf("arch.Lower produced diagnostics: %v", diags)
	}

	origReq := protocol.GenerationRequest{
		ProtocolVersion: protocol.ProtocolVersion,
		WorkspaceRoot:   "/workspace/shop",
		Target:          protocol.TargetInfoFromConfig(bp.Targets[0], map[string]string{"env": "test"}),
		SemanticIR:      sem,
		ArchitectureIR:  archIR,
		Files: map[string]string{
			"existing.go": "// pre-existing file",
		},
	}

	data, err := protocol.EncodeRequest(origReq)
	if err != nil {
		t.Fatalf("EncodeRequest failed: %v", err)
	}

	decodedReq, err := protocol.DecodeRequest(data)
	if err != nil {
		t.Fatalf("DecodeRequest failed: %v", err)
	}

	origJSON, err := json.Marshal(origReq)
	if err != nil {
		t.Fatalf("Marshal origReq failed: %v", err)
	}
	decodedJSON, err := json.Marshal(decodedReq)
	if err != nil {
		t.Fatalf("Marshal decodedReq failed: %v", err)
	}

	if string(origJSON) != string(decodedJSON) {
		t.Fatalf("Round-trip JSON mismatch.\nOriginal:\n%s\nDecoded:\n%s", origJSON, decodedJSON)
	}
}

func TestGenerationRequest_DefaultsAndValidation(t *testing.T) {
	data := []byte(`{
		"protocol_version": 1,
		"workspace_root": "/workspace",
		"target": {"name": "test", "kind": "backend", "language": "go", "out_dir": "gen"},
		"semantic_ir": {}
	}`)

	req, err := protocol.DecodeRequest(data)
	if err != nil {
		t.Fatalf("DecodeRequest failed: %v", err)
	}
	if req.Files == nil {
		t.Fatal("expected req.Files to be initialized to non-nil map")
	}
	if req.Target.Options == nil {
		t.Fatal("expected req.Target.Options to be initialized to non-nil map")
	}
	if err := req.Validate(); err != nil {
		t.Fatalf("expected valid request, got: %v", err)
	}

	// Missing workspace_root fails validation
	invalidReq := req
	invalidReq.WorkspaceRoot = ""
	if err := invalidReq.Validate(); err == nil {
		t.Error("expected error for empty WorkspaceRoot")
	}

	// Missing target.name fails validation
	invalidReq = req
	invalidReq.Target.Name = ""
	if err := invalidReq.Validate(); err == nil {
		t.Error("expected error for empty Target.Name")
	}

	// Missing target.kind fails validation
	invalidReq = req
	invalidReq.Target.Kind = ""
	if err := invalidReq.Validate(); err == nil {
		t.Error("expected error for empty Target.Kind")
	}

	// Missing target.language fails validation
	invalidReq = req
	invalidReq.Target.Language = ""
	if err := invalidReq.Validate(); err == nil {
		t.Error("expected error for empty Target.Language")
	}

	// Missing target.out_dir fails validation
	invalidReq = req
	invalidReq.Target.OutDir = ""
	if err := invalidReq.Validate(); err == nil {
		t.Error("expected error for empty Target.OutDir")
	}

	// Negative protocol version fails DecodeRequest
	negativeData := []byte(`{
		"protocol_version": -1,
		"workspace_root": "/workspace",
		"target": {"name": "test", "kind": "backend", "language": "go", "out_dir": "gen"},
		"semantic_ir": {}
	}`)
	if _, err := protocol.DecodeRequest(negativeData); err == nil {
		t.Error("expected error for negative protocol_version -1, got nil")
	}
	invalidReq = req
	invalidReq.SemanticIR = nil
	if err := invalidReq.Validate(); err == nil {
		t.Error("expected error for nil SemanticIR")
	}

	// Invalid negative or zero protocol version
	invalidReq = req
	invalidReq.ProtocolVersion = 0
	if err := invalidReq.Validate(); err == nil {
		t.Error("expected error for zero ProtocolVersion")
	}

	// Future protocol version fails validation and DecodeRequest
	futureData := []byte(`{
		"protocol_version": 99,
		"workspace_root": "/workspace",
		"target": {"name": "test", "kind": "backend", "language": "go", "out_dir": "gen"},
		"semantic_ir": {}
	}`)
	if _, err := protocol.DecodeRequest(futureData); err == nil {
		t.Fatal("expected error for future protocol_version 99, got nil")
	}

	// Missing protocol_version defaults to current ProtocolVersion for backward compatibility
	noVersionData := []byte(`{
		"workspace_root": "/workspace",
		"target": {"name": "test", "kind": "backend", "language": "go", "out_dir": "gen"},
		"semantic_ir": {}
	}`)
	reqNoVersion, err := protocol.DecodeRequest(noVersionData)
	if err != nil {
		t.Fatalf("unexpected error decoding payload without protocol_version: %v", err)
	}
	if reqNoVersion.ProtocolVersion != protocol.ProtocolVersion {
		t.Fatalf("expected defaulted ProtocolVersion %d, got %d", protocol.ProtocolVersion, reqNoVersion.ProtocolVersion)
	}
}

func TestGenerationRequest_FileHelpers(t *testing.T) {
	req := protocol.GenerationRequest{}
	if content, ok := req.GetFile("notfound.go"); ok || content != "" {
		t.Errorf("expected false for missing file in nil map, got ok=%v content=%q", ok, content)
	}

	req.SetFile("main.go", "package main")
	content, ok := req.GetFile("main.go")
	if !ok || content != "package main" {
		t.Errorf("expected package main, got ok=%v content=%q", ok, content)
	}
}

func TestGenerationResponse_RoundTripAndDefaults(t *testing.T) {
	resp := protocol.GenerationResponse{
		Files: map[string]string{
			"src/domain/order.go": "package domain",
		},
		Stubs: map[string]string{
			"src/slots/payment.go": "package slots",
		},
		Diagnostics: []ir.Diagnostic{
			{
				Code:     "FDN0702",
				Severity: ir.SeverityWarning,
				Message:  "file collision detected",
				Span:     ir.Span{File: "order.go", Line: 1, Column: 1},
			},
		},
		TestCommand: "go test ./...",
	}

	data, err := protocol.EncodeResponse(resp)
	if err != nil {
		t.Fatalf("EncodeResponse failed: %v", err)
	}

	decoded, err := protocol.DecodeResponse(data)
	if err != nil {
		t.Fatalf("DecodeResponse failed: %v", err)
	}

	if decoded.Files["src/domain/order.go"] != resp.Files["src/domain/order.go"] {
		t.Errorf("Files mismatch: got %+v", decoded.Files)
	}
	if decoded.Stubs["src/slots/payment.go"] != resp.Stubs["src/slots/payment.go"] {
		t.Errorf("Stubs mismatch: got %+v", decoded.Stubs)
	}
	if len(decoded.Diagnostics) != 1 || decoded.Diagnostics[0].Code != "FDN0702" {
		t.Errorf("Diagnostics mismatch: got %+v", decoded.Diagnostics)
	}
	if decoded.TestCommand != "go test ./..." {
		t.Errorf("TestCommand mismatch: got %q", decoded.TestCommand)
	}

	// Byte-for-byte JSON round-trip equality
	origJSON, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal resp failed: %v", err)
	}
	decodedJSON, err := json.Marshal(decoded)
	if err != nil {
		t.Fatalf("Marshal decoded failed: %v", err)
	}
	if string(origJSON) != string(decodedJSON) {
		t.Fatalf("Response round-trip JSON mismatch.\nOriginal:\n%s\nDecoded:\n%s", origJSON, decodedJSON)
	}

	// Test zero-value defaults on empty response payload
	emptyData := []byte(`{}`)
	emptyDecoded, err := protocol.DecodeResponse(emptyData)
	if err != nil {
		t.Fatalf("DecodeResponse empty failed: %v", err)
	}
	if emptyDecoded.Files == nil {
		t.Error("expected non-nil Files map on empty decoded response")
	}
	if emptyDecoded.Stubs == nil {
		t.Error("expected non-nil Stubs map on empty decoded response")
	}
	if emptyDecoded.Diagnostics == nil {
		t.Error("expected non-nil Diagnostics slice on empty decoded response")
	}
}
