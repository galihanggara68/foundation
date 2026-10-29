package lsp

import (
	"strings"
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
	foundSlot := false
	for _, item := range list.Items {
		if item.Label == "entity" {
			foundEntity = true
		}
		if item.Label == "action" {
			foundAction = true
		}
		if item.Label == "slot" {
			foundSlot = true
		}
	}

	if !foundEntity || !foundAction || !foundSlot {
		t.Fatalf("expected 'entity', 'action', and 'slot' in completion items, foundEntity=%v foundAction=%v foundSlot=%v", foundEntity, foundAction, foundSlot)
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

func TestComputeCompletion_DeclaresSnippetFormat(t *testing.T) {
	list := ComputeCompletion("action", Position{Line: 0, Character: 6})
	if list == nil || len(list.Items) == 0 {
		t.Fatalf("expected completion items, got none")
	}

	var action *CompletionItem
	for i := range list.Items {
		if list.Items[i].Label == "action" {
			action = &list.Items[i]
		}
	}
	if action == nil {
		t.Fatalf("expected an 'action' completion item")
	}
	if action.InsertTextFormat != InsertTextFormatSnippet {
		t.Errorf("action insertTextFormat = %d, want %d (snippet)", action.InsertTextFormat, InsertTextFormatSnippet)
	}
	if !strings.Contains(action.InsertText, "${1:Name}") {
		t.Errorf("expected a placeholder in action insertText, got %q", action.InsertText)
	}

	// Any item that carries snippet placeholders must declare the snippet format,
	// otherwise clients insert the placeholders literally.
	for _, item := range list.Items {
		if strings.Contains(item.InsertText, "$") && item.InsertTextFormat != InsertTextFormatSnippet {
			t.Errorf("%q contains snippet placeholders but declares insertTextFormat %d", item.Label, item.InsertTextFormat)
		}
	}
}
