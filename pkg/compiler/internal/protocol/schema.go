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
        "values": { "type": "array", "items": { "type": "object" } },
        "entities": { "type": "array", "items": { "type": "object" } },
        "errors": { "type": "array", "items": { "type": "object" } },
        "events": { "type": "array", "items": { "type": "object" } },
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
        "domain_errors": { "type": "array", "items": { "type": "object" } },
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
