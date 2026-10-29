package lsp

import (
	"strings"
	"testing"
)

const memberBlueprint = `entity Wallet {
  id: uuid
  balance: decimal(12, 2)
}

value TransferInput {
  to_wallet_id: uuid
  amount: decimal(12, 2)
}

action TransferMoney(input: TransferInput) -> bool {
  let sender = query one Wallet where id == input.to_wallet_id else WalletNotFound
  return true
}
`

// memberAccessDoc appends member-access lines so they can be completed against
// the symbols indexed from memberBlueprint (which stays at the same line numbers).
const memberAccessDoc = memberBlueprint + "  sender.\n  input.\n  sender.ba\n  unknown.\n  12.\n"

func memberIndexes(t *testing.T) (*DocumentIndex, []*DocumentIndex) {
	t.Helper()
	const uri = "file:///ws/members.fdn"
	idx := IndexDocument(uri, "members.fdn", memberBlueprint)
	if len(idx.Symbols) == 0 {
		t.Fatalf("expected symbols to be indexed")
	}
	return idx, []*DocumentIndex{idx}
}

func labels(list *CompletionList) []string {
	if list == nil {
		return nil
	}
	out := make([]string, 0, len(list.Items))
	for _, item := range list.Items {
		out = append(out, item.Label)
	}
	return out
}

func TestComputeCompletionAt_MemberAccess(t *testing.T) {
	idx, indexes := memberIndexes(t)

	cases := []struct {
		name    string
		line    int // 0-based
		char    int
		want    []string
		notWant string
	}{
		{"let variable holding an entity", 14, 9, []string{"id", "balance"}, ""},
		{"action parameter holding a value", 15, 8, []string{"to_wallet_id", "amount"}, ""},
		{"partial member prefix", 16, 11, []string{"balance"}, "id"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			list := ComputeCompletionAt(indexes, idx.URI, memberAccessDoc, Position{Line: tc.line, Character: tc.char})
			got := labels(list)

			if len(got) == 0 {
				t.Fatalf("expected member completions, got none")
			}
			for _, want := range tc.want {
				found := false
				for _, label := range got {
					if label == want {
						found = true
					}
				}
				if !found {
					t.Fatalf("expected %q in %v", want, got)
				}
			}
			if tc.notWant != "" {
				for _, label := range got {
					if label == tc.notWant {
						t.Fatalf("did not expect %q in %v", tc.notWant, got)
					}
				}
			}
			for _, item := range list.Items {
				if item.Kind != CompletionKindField {
					t.Errorf("%q: kind = %d, want field (%d)", item.Label, item.Kind, CompletionKindField)
				}
				if item.Detail == "" {
					t.Errorf("%q: expected a type detail", item.Label)
				}
			}
		})
	}
}

func TestComputeCompletionAt_UnknownReceiverReturnsNoMembers(t *testing.T) {
	idx, indexes := memberIndexes(t)

	list := ComputeCompletionAt(indexes, idx.URI, memberAccessDoc, Position{Line: 17, Character: 10})
	if got := labels(list); len(got) != 0 {
		t.Fatalf("expected no completions for an unresolved receiver, got %v", got)
	}
}

func TestComputeCompletionAt_NumericLiteralIsNotMemberAccess(t *testing.T) {
	idx, indexes := memberIndexes(t)

	list := ComputeCompletionAt(indexes, idx.URI, memberAccessDoc, Position{Line: 18, Character: 5})
	got := labels(list)
	if len(got) == 0 {
		t.Fatalf("expected the keyword catalog for a numeric literal, got none")
	}
	found := false
	for _, label := range got {
		if label == "entity" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected the keyword catalog fallback, got %v", got)
	}
}

func TestMemberContextAt(t *testing.T) {
	cases := []struct {
		line     string
		char     int
		receiver string
		prefix   string
		ok       bool
	}{
		{"  sender.", 9, "sender", "", true},
		{"  sender.ba", 11, "sender", "ba", true},
		{"  input.", 8, "input", "", true},
		{"a.b.", 4, "b", "", true},
		{"12.", 3, "", "", false},
		{"sender", 6, "", "", false},
		{"= .active", 9, "", "", false},
		{strings.Repeat(" ", 4) + "x", 5, "", "", false},
	}

	for _, tc := range cases {
		receiver, prefix, ok := memberContextAt(tc.line, Position{Line: 0, Character: tc.char})
		if ok != tc.ok || receiver != tc.receiver || prefix != tc.prefix {
			t.Errorf("%q@%d: got (%q, %q, %v), want (%q, %q, %v)",
				tc.line, tc.char, receiver, prefix, ok, tc.receiver, tc.prefix, tc.ok)
		}
	}
}

func TestComputeCompletionAt_RecoversTypeWhenDocumentDoesNotParse(t *testing.T) {
	// `sender.` is not a valid statement, so the parser discards the whole action
	// and the index has no `sender` binding; member completion must recover the
	// type from the source text so it keeps working while the user is typing.
	const doc = `entity Wallet {
  id: uuid
  balance: decimal(12, 2)
}

action TransferMoney(input: TransferInput) -> bool {
  let sender = query one Wallet where id == input.id else Missing
  sender.
  return true
}
`
	const uri = "file:///ws/typing.fdn"
	idx := IndexDocument(uri, "typing.fdn", doc)

	list := ComputeCompletionAt([]*DocumentIndex{idx}, uri, doc, Position{Line: 7, Character: 9})
	got := labels(list)
	if len(got) != 2 || got[0] != "id" || got[1] != "balance" {
		t.Fatalf("expected Wallet fields from the textual fallback, got %v", got)
	}
}

func TestFallbackLocalType(t *testing.T) {
	const doc = "action A(input: TransferInput) -> bool {\n  let sender = query one Wallet where id == input.id else Missing\n"
	cases := map[string]string{
		"sender": "Wallet",
		"input":  "TransferInput",
		"nope":   "",
	}
	for receiver, want := range cases {
		if got := fallbackLocalType(doc, receiver, 2); got != want {
			t.Errorf("fallbackLocalType(%q) = %q, want %q", receiver, got, want)
		}
	}
}
