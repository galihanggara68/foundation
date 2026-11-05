package protocol

// irDeclarationsSchemaJSON is the shared "definitions" member of the GenerationRequest schema.
// It defines every IR declaration so non-Go clients can generate typed models.
const irDeclarationsSchemaJSON = `  "definitions": {
    "FieldDecl": {
      "type": "object",
      "description": "A named, type-bearing field declared on an entity, value, or event.",
      "required": [
        "name",
        "type"
      ],
      "properties": {
        "name": {
          "type": "string"
        },
        "type": {
          "type": "string",
          "description": "Declared type string, including any array-ness (e.g. 'Customer' or '[]string'). The kind below classifies its element type."
        },
        "kind": {
          "type": "string",
          "description": "Closed, compiler-validated field kind. Optional: v1 documents produced before the taxonomy existed omit it.",
          "enum": [
            "uuid",
            "string",
            "int",
            "decimal",
            "bool",
            "timestamp",
            "enum",
            "entity",
            "value",
            "event",
            "actor",
            "page",
            "result"
          ]
        },
        "optional": {
          "type": "boolean"
        }
      }
    },
    "ErrorDecl": {
      "type": "object",
      "description": "A declared domain error: what it is called, how it is classified, and the text a caller may see.",
      "required": [
        "name",
        "kind",
        "message"
      ],
      "properties": {
        "name": {
          "type": "string"
        },
        "kind": {
          "type": "string",
          "description": "Closed, compiler-validated error kind, drawn from the platform's standard vocabulary.",
          "enum": [
            "not_found",
            "validation",
            "conflict",
            "unauthenticated",
            "unauthorized",
            "forbidden",
            "internal"
          ]
        },
        "message": {
          "type": "string",
          "description": "Fallback text for an error with no locale-specific message; empty when a declaration carries only 'messages'."
        },
        "messages": {
          "type": "object",
          "description": "Text per locale tag (e.g. \"en\", \"en-US\"). Optional: v1 documents produced before locale messages existed omit it.",
          "additionalProperties": {
            "type": "string"
          }
        },
        "status": {
          "type": "integer",
          "description": "HTTP status the transport answers with, resolved by the compiler: a declared status wins, otherwise the kind's standard status. Optional: v1 documents produced before resolution omit it."
        },
        "posture": {
          "type": "string",
          "description": "Whether the message is caller-facing text or an internal diagnostic, resolved by the compiler. Optional: v1 documents produced before posture existed omit it.",
          "enum": [
            "business",
            "technical"
          ]
        }
      }
    },
    "Span": {
      "type": "object",
      "description": "Source location of a declaration or expression.",
      "required": [ "file", "line", "column" ],
      "properties": {
        "file": { "type": "string" },
        "line": { "type": "integer" },
        "column": { "type": "integer" }
      }
    },
    "TraitDecl": {
      "type": "object",
      "description": "A metadata annotation carried by a declaration, field, or slot method.",
      "required": [ "name" ],
      "properties": {
        "name": { "type": "string" },
        "args": { "type": "object", "additionalProperties": { "type": "string" } },
        "list_args": { "type": "object", "additionalProperties": { "type": "array", "items": { "type": "string" } } }
      }
    },
    "ParamDecl": {
      "type": "object",
      "description": "A declared action or slot-method parameter.",
      "required": [ "name", "type" ],
      "properties": {
        "name": { "type": "string" },
        "type": { "type": "string" },
        "kind": { "type": "string", "description": "Compiler-validated field kind of the parameter." },
        "in_path": { "type": "boolean" }
      }
    },
    "EnumDecl": {
      "type": "object",
      "description": "A closed set of named string variants.",
      "required": [ "name" ],
      "properties": {
        "name": { "type": "string" },
        "variants": { "type": "array", "items": { "type": "string" } }
      }
    },
    "ValueDecl": {
      "type": "object",
      "description": "An immutable data structure with no identity.",
      "required": [ "name" ],
      "properties": {
        "name": { "type": "string" },
        "fields": { "type": "array", "items": { "$ref": "#/definitions/FieldDecl" } }
      }
    },
    "EntityDecl": {
      "type": "object",
      "description": "A persistent domain entity and its physical table mapping.",
      "required": [ "name" ],
      "properties": {
        "name": { "type": "string" },
        "table_name": { "type": "string", "description": "Physical table name the compiler assigns to the entity." },
        "fields": { "type": "array", "items": { "$ref": "#/definitions/FieldDecl" } },
        "traits": { "type": "array", "items": { "$ref": "#/definitions/TraitDecl" } }
      }
    },
    "EventDecl": {
      "type": "object",
      "description": "A domain occurrence an action emits after a state change.",
      "required": [ "name" ],
      "properties": {
        "name": { "type": "string" },
        "fields": { "type": "array", "items": { "$ref": "#/definitions/FieldDecl" } }
      }
    },
    "ActorDecl": {
      "type": "object",
      "description": "The principal that requests an action.",
      "required": [ "name" ],
      "properties": {
        "name": { "type": "string" },
        "fields": { "type": "array", "items": { "$ref": "#/definitions/FieldDecl" } }
      }
    },
    "PolicyDecl": {
      "type": "object",
      "description": "A reusable boolean rule with no side effects.",
      "required": [ "name" ],
      "properties": {
        "name": { "type": "string" },
        "params": { "type": "array", "items": { "$ref": "#/definitions/ParamDecl" } },
        "body": { "$ref": "#/definitions/Expr" }
      }
    },
    "SlotMethodDecl": {
      "type": "object",
      "description": "A method on a typed extension slot.",
      "required": [ "name" ],
      "properties": {
        "name": { "type": "string" },
        "params": { "type": "array", "items": { "$ref": "#/definitions/ParamDecl" } },
        "return_type": { "type": "string" },
        "return_kind": { "type": "string" },
        "throws": { "type": "array", "items": { "type": "string" } },
        "traits": { "type": "array", "items": { "$ref": "#/definitions/TraitDecl" } }
      }
    },
    "SlotDecl": {
      "type": "object",
      "description": "A typed extension contract in the blueprint. The compiler materializes derived slots.",
      "required": [ "name" ],
      "properties": {
        "name": { "type": "string" },
        "methods": { "type": "array", "items": { "$ref": "#/definitions/SlotMethodDecl" } },
        "derived": { "type": "boolean", "description": "True when the compiler materialized the slot as an implied capability. Optional." }
      }
    },
    "ActionDecl": {
      "type": "object",
      "description": "An action pipeline: declared parameters, return type, traits, ordered steps, and the HTTP route binding from the @http trait.",
      "required": [ "name" ],
      "properties": {
        "name": { "type": "string" },
        "params": { "type": "array", "items": { "$ref": "#/definitions/ParamDecl" } },
        "return_type": { "type": "string" },
        "traits": { "type": "array", "items": { "$ref": "#/definitions/TraitDecl" } },
        "steps": { "type": "array", "items": { "$ref": "#/definitions/StepDecl" } }
      }
    },
    "FieldInitDecl": {
      "type": "object",
      "description": "A named field assignment inside a construct or match expression.",
      "required": [ "name" ],
      "properties": {
        "name": { "type": "string" },
        "value": { "$ref": "#/definitions/Expr" }
      }
    },
    "WherePredicate": {
      "type": "object",
      "description": "One condition in a query where-clause.",
      "required": [ "field", "op" ],
      "properties": {
        "field": { "type": "string" },
        "op": { "type": "string" },
        "value": { "$ref": "#/definitions/Expr" }
      }
    },
    "MatchArmDecl": {
      "type": "object",
      "description": "One arm of a match step.",
      "required": [ "variant" ],
      "properties": {
        "variant": { "type": "string" },
        "steps": { "type": "array", "items": { "$ref": "#/definitions/StepDecl" } }
      }
    },
    "Expr": {
      "type": "object",
      "description": "A typed expression node; the 'kind' field discriminates the shape.",
      "required": [ "kind" ],
      "properties": {
        "kind": {
          "type": "string",
          "enum": [ "ident", "field", "string", "int", "float", "bool", "enum_lit", "binary", "construct", "list", "combinator", "call" ]
        },
        "value": { "type": "string" },
        "target": { "type": "string" },
        "field": { "type": "string" },
        "op": { "type": "string" },
        "left": { "$ref": "#/definitions/Expr" },
        "right": { "$ref": "#/definitions/Expr" },
        "type_name": { "type": "string" },
        "fields": { "type": "array", "items": { "$ref": "#/definitions/FieldInitDecl" } },
        "items": { "type": "array", "items": { "$ref": "#/definitions/Expr" } },
        "inferred_type": { "type": "string" }
      }
    },
    "StepDecl": {
      "type": "object",
      "description": "One action-pipeline step. The 'kind' field discriminates which optional fields apply.",
      "required": [ "kind" ],
      "properties": {
        "kind": {
          "type": "string",
          "description": "Step discriminator.",
          "enum": [ "let", "query", "query_many", "create", "update", "delete", "require", "guard", "emit", "slot", "if", "match", "for", "while", "return" ]
        },
        "var_name": { "type": "string" },
        "entity_name": { "type": "string" },
        "slot_name": { "type": "string" },
        "slot_method": { "type": "string" },
        "slot_args": { "type": "array", "items": { "$ref": "#/definitions/Expr" } },
        "policy_name": { "type": "string" },
        "policy_args": { "type": "array", "items": { "$ref": "#/definitions/Expr" } },
        "event_name": { "type": "string" },
        "where_field": { "type": "string" },
        "where_value": { "$ref": "#/definitions/Expr" },
        "where_predicates": { "type": "array", "items": { "$ref": "#/definitions/WherePredicate" } },
        "where_expr": { "$ref": "#/definitions/Expr" },
        "order_by_field": { "type": "string" },
        "order_by_desc": { "type": "boolean" },
        "limit": { "type": "integer" },
        "limit_expr": { "$ref": "#/definitions/Expr" },
        "cursor_expr": { "$ref": "#/definitions/Expr" },
        "else_error": { "type": "string" },
        "fields": { "type": "array", "items": { "$ref": "#/definitions/FieldInitDecl" } },
        "expr": { "$ref": "#/definitions/Expr" },
        "iter_var": { "type": "string" },
        "then_steps": { "type": "array", "items": { "$ref": "#/definitions/StepDecl" } },
        "else_steps": { "type": "array", "items": { "$ref": "#/definitions/StepDecl" } },
        "match_arms": { "type": "array", "items": { "$ref": "#/definitions/MatchArmDecl" } }
      },
      "oneOf": [
        { "properties": { "kind": { "const": "let" } }, "required": [ "kind" ] },
        { "properties": { "kind": { "const": "query" } }, "required": [ "kind" ] },
        { "properties": { "kind": { "const": "query_many" } }, "required": [ "kind" ] },
        { "properties": { "kind": { "const": "create" } }, "required": [ "kind" ] },
        { "properties": { "kind": { "const": "update" } }, "required": [ "kind" ] },
        { "properties": { "kind": { "const": "delete" } }, "required": [ "kind" ] },
        { "properties": { "kind": { "const": "require" } }, "required": [ "kind" ] },
        { "properties": { "kind": { "const": "guard" } }, "required": [ "kind" ] },
        { "properties": { "kind": { "const": "emit" } }, "required": [ "kind" ] },
        { "properties": { "kind": { "const": "slot" } }, "required": [ "kind" ] },
        { "properties": { "kind": { "const": "if" } }, "required": [ "kind" ] },
        { "properties": { "kind": { "const": "match" } }, "required": [ "kind" ] },
        { "properties": { "kind": { "const": "for" } }, "required": [ "kind" ] },
        { "properties": { "kind": { "const": "while" } }, "required": [ "kind" ] },
        { "properties": { "kind": { "const": "return" } }, "required": [ "kind" ] }
      ]
    },
    "GivenSeedDecl": {
      "type": "object",
      "description": "Seed data a scenario places before it runs.",
      "required": [ "entity_name" ],
      "properties": {
        "entity_name": { "type": "string" },
        "fields": { "type": "array", "items": { "$ref": "#/definitions/FieldInitDecl" } }
      }
    },
    "GivenSlotDecl": {
      "type": "object",
      "description": "A slot invocation a scenario arranges before it runs.",
      "required": [ "slot_name", "method_name" ],
      "properties": {
        "slot_name": { "type": "string" },
        "method_name": { "type": "string" },
        "args": { "type": "array", "items": { "$ref": "#/definitions/Expr" } },
        "return_expr": { "$ref": "#/definitions/Expr" },
        "throws_error": { "type": "string" }
      }
    },
    "ExpectDecl": {
      "type": "object",
      "description": "One scenario assertion.",
      "properties": {
        "left": { "$ref": "#/definitions/Expr" },
        "op": { "type": "string" },
        "right": { "$ref": "#/definitions/Expr" },
        "error_raised": { "type": "string" },
        "emitted_event": { "type": "string" },
        "emitted_fields": { "type": "array", "items": { "$ref": "#/definitions/FieldInitDecl" } },
        "stored_entity": { "type": "string" },
        "where_field": { "type": "string" },
        "where_value": { "$ref": "#/definitions/Expr" },
        "where_predicates": { "type": "array", "items": { "$ref": "#/definitions/WherePredicate" } },
        "where_expr": { "$ref": "#/definitions/Expr" },
        "stored_fields": { "type": "array", "items": { "$ref": "#/definitions/FieldInitDecl" } },
        "not_stored": { "type": "boolean" }
      }
    },
    "ScenarioDecl": {
      "type": "object",
      "description": "A declarative test scenario over an action.",
      "required": [ "name", "when_action" ],
      "properties": {
        "name": { "type": "string" },
        "givens": { "type": "array", "items": { "$ref": "#/definitions/GivenSeedDecl" } },
        "given_slots": { "type": "array", "items": { "$ref": "#/definitions/GivenSlotDecl" } },
        "when_action": { "type": "string" },
        "when_args": { "type": "array", "items": { "$ref": "#/definitions/Expr" } },
        "expects": { "type": "array", "items": { "$ref": "#/definitions/ExpectDecl" } }
      }
    }
  }`
