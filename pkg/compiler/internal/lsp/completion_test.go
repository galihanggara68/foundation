package lsp

import (
	"testing"
)

func TestComputeCompletion_TopLevelKeywords(t *testing.T) {
	doc := ""
	list := ComputeCompletion(doc, Position{Line: 0, Character: 0})
	if list == nil || len(list.Items) == 0 {
		t.Fatalf("expected top-level completion items, got none")
	}

	foundEntity := false
	foundAction := false
	for _, item := range list.Items {
		if item.Label == "entity" {
			foundEntity = true
		}
		if item.Label == "action" {
			foundAction = true
		}
	}

	if !foundEntity || !foundAction {
		t.Fatalf("expected 'entity' and 'action' in completion items, foundEntity=%v foundAction=%v", foundEntity, foundAction)
	}
}

func TestComputeCompletion_TraitsWithAtPrefix(t *testing.T) {
	doc := "entity Order @"
	list := ComputeCompletion(doc, Position{Line: 0, Character: 14})
	if list == nil || len(list.Items) == 0 {
		t.Fatalf("expected trait completion items, got none")
	}

	foundTable := false
	foundVersion := false
	for _, item := range list.Items {
		if item.Label == "@table" {
			foundTable = true
		}
		if item.Label == "@version" {
			foundVersion = true
		}
		if item.Kind == CompletionKindKeyword {
			t.Fatalf("unexpected keyword %q in trait completion context", item.Label)
		}
	}

	if !foundTable || !foundVersion {
		t.Fatalf("expected @table and @version in trait completions, foundTable=%v foundVersion=%v", foundTable, foundVersion)
	}
}

func TestComputeCompletion_ScalarTypes(t *testing.T) {
	doc := "entity User {\n  id: \n}"
	list := ComputeCompletion(doc, Position{Line: 1, Character: 6})
	if list == nil || len(list.Items) == 0 {
		t.Fatalf("expected completion items, got none")
	}

	foundUUID := false
	foundMoney := false
	for _, item := range list.Items {
		if item.Label == "UUID" {
			foundUUID = true
		}
		if item.Label == "Money" {
			foundMoney = true
		}
	}
	if !foundUUID || !foundMoney {
		t.Fatalf("expected 'UUID' and 'Money' in type completions, foundUUID=%v foundMoney=%v", foundUUID, foundMoney)
	}
}
