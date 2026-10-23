package ir

type StepKind string

const (
	StepLet     StepKind = "let"
	StepQuery   StepKind = "query"
	StepCreate  StepKind = "create"
	StepUpdate  StepKind = "update"
	StepDelete  StepKind = "delete"
	StepRequire StepKind = "require"
	StepGuard   StepKind = "guard"
	StepEmit    StepKind = "emit"
	StepSlot    StepKind = "slot"
	StepIf      StepKind = "if"
	StepMatch   StepKind = "match"
	StepFor     StepKind = "for"
	StepWhile   StepKind = "while"
	StepReturn  StepKind = "return"
)

type ExprKind string

const (
	ExprIdent      ExprKind = "ident"
	ExprField      ExprKind = "field"
	ExprString     ExprKind = "string"
	ExprInt        ExprKind = "int"
	ExprFloat      ExprKind = "float"
	ExprBool       ExprKind = "bool"
	ExprEnumLit    ExprKind = "enum_lit"
	ExprBinary     ExprKind = "binary"
	ExprConstruct  ExprKind = "construct"
	ExprList       ExprKind = "list"
	ExprCombinator ExprKind = "combinator"
	ExprCall       ExprKind = "call"
)

type Expr struct {
	Kind         ExprKind        `json:"kind"`
	Value        string          `json:"value,omitempty"`
	Target       string          `json:"target,omitempty"`
	Field        string          `json:"field,omitempty"`
	Op           string          `json:"op,omitempty"`
	Left         *Expr           `json:"left,omitempty"`
	Right        *Expr           `json:"right,omitempty"`
	TypeName     string          `json:"type_name,omitempty"`
	Fields       []FieldInitDecl `json:"fields,omitempty"`
	Items        []Expr          `json:"items,omitempty"`
	InferredType string          `json:"inferred_type,omitempty"`
	Span         Span            `json:"span"`
}

type FieldInitDecl struct {
	Name  string `json:"name"`
	Value Expr   `json:"value"`
	Span  Span   `json:"span"`
}

type MatchArmDecl struct {
	Variant string     `json:"variant"`
	Steps   []StepDecl `json:"steps"`
	Span    Span       `json:"span"`
}

type StepDecl struct {
	Kind       StepKind        `json:"kind"`
	VarName    string          `json:"var_name,omitempty"`
	EntityName string          `json:"entity_name,omitempty"`
	SlotName   string          `json:"slot_name,omitempty"`
	SlotMethod string          `json:"slot_method,omitempty"`
	SlotArgs   []Expr          `json:"slot_args,omitempty"`
	PolicyName string          `json:"policy_name,omitempty"`
	PolicyArgs []Expr          `json:"policy_args,omitempty"`
	EventName  string          `json:"event_name,omitempty"`
	WhereField string          `json:"where_field,omitempty"`
	WhereValue *Expr           `json:"where_value,omitempty"`
	ElseError  string          `json:"else_error,omitempty"`
	Fields     []FieldInitDecl `json:"fields,omitempty"`
	Expr       *Expr           `json:"expr,omitempty"`
	IterVar    string          `json:"iter_var,omitempty"`
	ThenSteps  []StepDecl      `json:"then_steps,omitempty"`
	ElseSteps  []StepDecl      `json:"else_steps,omitempty"`
	MatchArms  []MatchArmDecl  `json:"match_arms,omitempty"`
	Span       Span            `json:"span"`
}
