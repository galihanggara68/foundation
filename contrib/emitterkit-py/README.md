# emitterkit-py

A zero-dependency reference client for the Foundation generation protocol.
Python plugin authors use the package to decode requests and to encode responses.

## Install

Add the `emitterkit` directory to your plugin project, or copy it into your source tree.

## Usage

```python
from emitterkit import GenerationRequest, GenerationResponse, run_plugin


def handle(req: GenerationRequest) -> GenerationResponse:
    resp = GenerationResponse(test_command="pytest")
    resp.files["generated.py"] = "# generated\n"
    return resp


if __name__ == "__main__":
    run_plugin(handle)
```

The compiler sends a `GenerationRequest` on standard input.
The plugin writes a `GenerationResponse` on standard output.
`run_plugin` keeps standard output pure for the wire protocol.

## API

- `decode_request(source)` decodes a `GenerationRequest` from JSON bytes, text, or a stream.
- `encode_response(resp)` encodes a `GenerationResponse` to a JSON string.
- `run_plugin(handler)` reads a request, runs the handler, and writes the response.

The published JSON Schema is at [`docs/schemas/generation-protocol-v1.json`](../../docs/schemas/generation-protocol-v1.json).

## Test

```bash
python3 -m unittest discover -s tests
```
