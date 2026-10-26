package tssdk

import (
	"fmt"
	"strings"

	"github.com/galihanggara68/foundation/pkg/compiler/internal/arch"
)

func emitClient(archIR *arch.ArchitectureIR) string {
	var sb strings.Builder
	sb.WriteString(`import * as types from "./types.ts";
import * as errors from "./errors.ts";

export interface ClientOptions {
  baseURL: string;
  authToken?: string;
  headers?: Record<string, string>;
  fetch?: typeof fetch;
}

export class Client {
  private readonly baseURL: string;
  private readonly authToken?: string;
  private readonly defaultHeaders: Record<string, string>;
  private readonly customFetch?: typeof fetch;

  constructor(options: ClientOptions) {
    this.baseURL = options.baseURL.replace(/\/+$/, "");
    this.authToken = options.authToken;
    this.defaultHeaders = options.headers || {};
    this.customFetch = options.fetch;
  }

  private async doRequest<T>(method: string, path: string, query?: Record<string, any>, body?: any): Promise<T> {
    let fullURL = this.baseURL + path;
    if (query && Object.keys(query).length > 0) {
      const searchParams = new URLSearchParams();
      for (const [k, v] of Object.entries(query)) {
        if (v !== undefined && v !== null && v !== "") {
          searchParams.set(k, String(v));
        }
      }
      const qs = searchParams.toString();
      if (qs.length > 0) {
        fullURL += "?" + qs;
      }
    }

    const headers: Record<string, string> = {
      "Accept": "application/json",
      ...this.defaultHeaders,
    };

    let bodyData: string | undefined = undefined;
    if (body !== undefined && body !== null) {
      headers["Content-Type"] = "application/json";
      bodyData = JSON.stringify(body);
    }

    if (this.authToken) {
      headers["Authorization"] = ` + "`Bearer ${this.authToken}`" + `;
    }

    const fetchFn = this.customFetch || globalThis.fetch;
    const response = await fetchFn(fullURL, {
      method,
      headers,
      body: bodyData,
    });

    if (!response.ok) {
      let errPayload: any = null;
      try {
        errPayload = await response.json();
      } catch {
        // non-json error body
      }
      throw errors.createError(response.status, errPayload);
    }

    if (response.status === 204) {
      return undefined as unknown as T;
    }

    return await response.json() as T;
  }

`)

	for _, h := range archIR.HTTPHandlers {
		methodName := toCamelCase(h.ActionName)
		inputTypeName := fmt.Sprintf("types.%sInput", h.ActionName)

		isVoid := h.ReturnType == "" || h.ReturnType == "void"
		isList := h.IsListReturning

		var retSig string
		if isVoid {
			retSig = "Promise<void>"
		} else if isList {
			retSig = fmt.Sprintf("Promise<types.%sResponse>", h.ActionName)
		} else {
			retSig = fmt.Sprintf("Promise<types.%s>", toTSType(h.ReturnType))
		}

		sb.WriteString(fmt.Sprintf("  async %s(input: %sInput): %s {\n", methodName, inputTypeName, retSig))
		sb.WriteString(fmt.Sprintf("    let path = %q;\n", h.Path))

		pathParamSet := make(map[string]bool)
		for _, pp := range h.PathParams {
			pathParamSet[pp] = true
			fName := arch.ToSnakeCase(pp)
			sb.WriteString(fmt.Sprintf("    path = path.replace(\"{%s}\", encodeURIComponent(String(input.%s)));\n", pp, fName))
		}

		if h.Method == "GET" {
			sb.WriteString("    const query: Record<string, any> = {};\n")
			for _, p := range h.Params {
				if pathParamSet[p.Name] {
					continue
				}
				fName := arch.ToSnakeCase(p.Name)
				sb.WriteString(fmt.Sprintf("    if (input.%s !== undefined) query[%q] = input.%s;\n", fName, p.Name, fName))
			}
			sb.WriteString("    const body = undefined;\n")
		} else {
			sb.WriteString("    const query = undefined;\n")
			sb.WriteString("    const body = input;\n")
		}

		if isVoid {
			sb.WriteString(fmt.Sprintf("    await this.doRequest<void>(%q, path, query, body);\n  }\n\n", h.Method))
		} else if isList {
			sb.WriteString(fmt.Sprintf("    return await this.doRequest<types.%sResponse>(%q, path, query, body);\n  }\n\n", h.ActionName, h.Method))
		} else {
			retType := toTSType(h.ReturnType)
			sb.WriteString(fmt.Sprintf("    return await this.doRequest<types.%s>(%q, path, query, body);\n  }\n\n", retType, h.Method))
		}
	}

	sb.WriteString("}\n")
	return sb.String()
}

func toCamelCase(s string) string {
	if s == "" {
		return ""
	}
	parts := strings.Split(s, "_")
	var sb strings.Builder
	for i, p := range parts {
		if p == "" {
			continue
		}
		if i == 0 {
			sb.WriteString(strings.ToLower(p[:1]) + p[1:])
		} else {
			sb.WriteString(strings.ToUpper(p[:1]) + p[1:])
		}
	}
	return sb.String()
}
