package protocol

// GenerationRequestSchemaJSON defines the JSON Schema Draft-07 specification for GenerationRequest.
// The compiler assembles the schema from a head fragment, the shared declaration definitions, and a closing brace.
const GenerationRequestSchemaJSON = schemaRequestHead + irDeclarationsSchemaJSON + schemaRequestTail

// schemaRequestHead is the GenerationRequest schema up to and including the comma after its
// "properties" member, immediately before the shared "definitions" member.
const schemaRequestHead = `{
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
        "actor": { "$ref": "#/definitions/ActorDecl" },
        "actors": { "type": "array", "items": { "$ref": "#/definitions/ActorDecl" } },
        "enums": { "type": "array", "items": { "$ref": "#/definitions/EnumDecl" } },
        "values": { "type": "array", "items": { "$ref": "#/definitions/ValueDecl" } },
        "entities": { "type": "array", "items": { "$ref": "#/definitions/EntityDecl" } },
        "errors": { "type": "array", "items": { "$ref": "#/definitions/ErrorDecl" } },
        "events": { "type": "array", "items": { "$ref": "#/definitions/EventDecl" } },
        "policies": { "type": "array", "items": { "$ref": "#/definitions/PolicyDecl" } },
        "slots": { "type": "array", "items": { "$ref": "#/definitions/SlotDecl" } },
        "actions": { "type": "array", "items": { "$ref": "#/definitions/ActionDecl" } },
        "scenarios": { "type": "array", "items": { "$ref": "#/definitions/ScenarioDecl" } }
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
        "domain_enums": { "type": "array", "items": { "$ref": "#/definitions/EnumDecl" } },
        "domain_values": { "type": "array", "items": { "$ref": "#/definitions/ValueDecl" } },
        "domain_entities": { "type": "array", "items": { "$ref": "#/definitions/EntityDecl" } },
        "domain_errors": { "type": "array", "items": { "$ref": "#/definitions/ErrorDecl" } },
        "domain_events": { "type": "array", "items": { "$ref": "#/definitions/EventDecl" } },
        "domain_policies": { "type": "array", "items": { "$ref": "#/definitions/PolicyDecl" } },
        "domain_actor": { "$ref": "#/definitions/ActorDecl" },
        "slots": { "type": "array", "items": { "$ref": "#/definitions/SlotDecl" } },
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
        },
        "dependencies": {
          "type": "array",
          "description": "Canonical third-party dependency versions the compiler publishes for downstream generators.",
          "items": {
            "type": "object",
            "required": [ "language", "package", "version" ],
            "properties": {
              "language": { "type": "string" },
              "package": { "type": "string" },
              "version": { "type": "string" }
            }
          }
        }
      }
    }
  },
`

// schemaRequestTail closes the GenerationRequest root object after the "definitions" member.
const schemaRequestTail = `
}
`

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
