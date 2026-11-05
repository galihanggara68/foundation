// Type definitions for the emitterkit-ts client.
// The runtime implementation lives in ./index.js; this file declares the typed surface.

export interface Span {
  file: string;
  line: number;
  column: number;
}

export interface Diagnostic {
  code: string;
  severity: string;
  message: string;
  span: Span;
}

export interface TargetInfo {
  name: string;
  kind: string;
  language: string;
  out_dir: string;
  architecture?: string;
  http?: string;
  options?: Record<string, string>;
}

export interface GenerationRequest {
  protocol_version: number;
  workspace_root: string;
  target: TargetInfo;
  semantic_ir: Record<string, unknown>;
  architecture_ir?: Record<string, unknown>;
  platform?: Record<string, unknown>;
  files?: Record<string, string>;
}

export interface GenerationResponse {
  protocol_version?: number;
  files?: Record<string, string>;
  stubs?: Record<string, string>;
  diagnostics?: Diagnostic[];
  test_command?: string;
}

export type PluginHandler = (req: GenerationRequest) => GenerationResponse | Promise<GenerationResponse>;

export interface PluginStreams {
  read?: () => string;
  write?: (chunk: string) => void;
}

export declare const PROTOCOL_VERSION: number;

// decodeRequest parses a GenerationRequest from JSON text or bytes.
export declare function decodeRequest(input: string | Uint8Array): GenerationRequest;

// encodeResponse serializes a GenerationResponse, omitting empty collections.
export declare function encodeResponse(resp: GenerationResponse): string;

// runPlugin reads a request, runs the handler, and writes the response.
// The function writes only the response document to standard output.
export declare function runPlugin(handler: PluginHandler, streams?: PluginStreams): Promise<number>;
