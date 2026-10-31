package protocol

// GenerationRequestSchemaJSON defines the JSON Schema Draft-07 specification for GenerationRequest.
const GenerationRequestSchemaJSON = `{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "GenerationRequest",
  "description": "Payload sent from the Foundation compiler to an external generator plugin.",
  "type": "object",
  "required": [
    "protocol_version",
    "workspace_root",
    "target",
    "semantic_ir"
  ],
  "properties": {
    "protocol_version": {
      "type": "integer",
      "description": "Version of the Foundation plugin generation wire protocol.",
      "minimum": 1
    },
    "workspace_root": {
      "type": "string",
      "description": "Absolute or relative path to the workspace root directory."
    },
    "target": {
      "type": "object",
      "description": "Target configuration and custom generator options.",
      "required": [
        "name",
        "kind",
        "language",
        "out_dir"
      ],
      "properties": {
        "name": { "type": "string" },
        "kind": { "type": "string" },
        "language": { "type": "string" },
        "architecture": { "type": "string" },
        "http": { "type": "string" },
        "out_dir": { "type": "string" },
        "options": {
          "type": "object",
          "additionalProperties": { "type": "string" }
        }
      }
    },
    "semantic_ir": {
      "type": "object",
      "description": "Validated semantic IR model containing domain definitions.",
      "properties": {
        "actor": { "type": "object" },
        "actors": { "type": "array", "items": { "type": "object" } },
        "enums": { "type": "array", "items": { "type": "object" } },
        "values": {
          "type": "array",
          "items": {
            "type": "object",
            "properties": {
              "name": { "type": "string" },
              "fields": {
                "type": "array",
                "items": { "$ref": "#/definitions/FieldDecl" }
              }
            }
          }
        },
        "entities": {
          "type": "array",
          "items": {
            "type": "object",
            "properties": {
              "name": { "type": "string" },
              "fields": {
                "type": "array",
                "items": { "$ref": "#/definitions/FieldDecl" }
              }
            }
          }
        },
        "errors": { "type": "array", "items": { "$ref": "#/definitions/ErrorDecl" } },
        "events": {
          "type": "array",
          "items": {
            "type": "object",
            "properties": {
              "name": { "type": "string" },
              "fields": {
                "type": "array",
                "items": { "$ref": "#/definitions/FieldDecl" }
              }
            }
          }
        },
        "policies": { "type": "array", "items": { "type": "object" } },
        "slots": { "type": "array", "items": { "type": "object" } },
        "actions": { "type": "array", "items": { "type": "object" } },
        "scenarios": { "type": "array", "items": { "type": "object" } }
      }
    },
    "architecture_ir": {
      "type": "object",
      "description": "Optional pre-lowered architecture IR model.",
      "properties": {
        "target_name": { "type": "string" },
        "kind": { "type": "string" },
        "language": { "type": "string" },
        "pattern": { "type": "string" },
        "http": { "type": "string" },
        "module": { "type": "string" },
        "out_dir": { "type": "string" },
        "storage": { "type": "object" },
        "layers": { "type": "array", "items": { "type": "object" } },
        "domain_enums": { "type": "array", "items": { "type": "object" } },
        "domain_values": { "type": "array", "items": { "type": "object" } },
        "domain_entities": { "type": "array", "items": { "type": "object" } },
        "domain_errors": { "type": "array", "items": { "$ref": "#/definitions/ErrorDecl" } },
        "domain_events": { "type": "array", "items": { "type": "object" } },
        "domain_policies": { "type": "array", "items": { "type": "object" } },
        "domain_actor": { "type": "object" },
        "slots": { "type": "array", "items": { "type": "object" } },
        "ports": { "type": "array", "items": { "type": "object" } },
        "use_cases": { "type": "array", "items": { "type": "object" } }
      }
    },
    "files": {
      "type": "object",
      "description": "Map of relative virtual file paths to their utf-8 string content.",
      "additionalProperties": { "type": "string" }
    },
    "platform": {
      "type": "object",
      "description": "Compiler-owned platform policy every emitter consumes. Optional: v1 requests produced before it existed omit it.",
      "properties": {
        "pagination": {
          "type": "object",
          "description": "Platform page-size policy for list reads.",
          "properties": {
            "default_limit": {
              "type": "integer",
              "description": "Page size to apply when a caller does not supply one."
            },
            "max_limit": {
              "type": "integer",
              "description": "Hard ceiling that any requested page size is clamped to."
            }
          }
        }
      }
    }
  },
  "definitions": {
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
    }
  }
}`

// GenerationResponseSchemaJSON defines the JSON Schema Draft-07 specification for GenerationResponse.
const GenerationResponseSchemaJSON = `{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "GenerationResponse",
  "description": "Payload returned from an external generator plugin to the Foundation compiler.",
  "type": "object",
  "properties": {
    "files": {
      "type": "object",
      "description": "Map of tracked relative file paths to their utf-8 string content.",
      "additionalProperties": { "type": "string" }
    },
    "stubs": {
      "type": "object",
      "description": "Map of create-if-missing stub file paths to their utf-8 string content.",
      "additionalProperties": { "type": "string" }
    },
    "diagnostics": {
      "type": "array",
      "description": "List of compilation diagnostics emitted by the generator.",
      "items": {
        "type": "object",
        "required": [
          "code",
          "severity",
          "message",
          "span"
        ],
        "properties": {
          "code": { "type": "string" },
          "severity": {
            "type": "string",
            "enum": [ "error", "warning" ]
          },
          "message": { "type": "string" },
          "span": {
            "type": "object",
            "required": [ "file", "line", "column" ],
            "properties": {
              "file": { "type": "string" },
              "line": { "type": "integer" },
              "column": { "type": "integer" }
            }
          }
        }
      }
    },
    "test_command": {
      "type": "string",
      "description": "Command string to run target integration tests (e.g. 'cargo test', 'pytest')."
    }
  }
}`

// RequestJSONSchema returns the JSON Schema Draft-07 definition for GenerationRequest.
func RequestJSONSchema() string {
	return GenerationRequestSchemaJSON
}

// ResponseJSONSchema returns the JSON Schema Draft-07 definition for GenerationResponse.
func ResponseJSONSchema() string {
	return GenerationResponseSchemaJSON
}
