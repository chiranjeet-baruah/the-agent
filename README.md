# the-agent

Minimal learning AI agent: [gin-gonic/gin](https://github.com/gin-gonic/gin) HTTP layer,
[google/adk-go](https://github.com/google/adk-go) agent framework, [Ollama](https://ollama.com)
as the local model backend.

## Prerequisites

- Docker (for Ollama)
- Go >= 1.26.5 — `google.golang.org/adk/v2` requires it. If your local `go version` is older
  and `go env GOTOOLCHAIN` is `local`, prefix Go commands with `GOTOOLCHAIN=go1.26.5` (Go will
  download that toolchain on first use). If `GOTOOLCHAIN` is `auto` (Go's default), no prefix
  is needed.

## Run

```sh
docker compose up -d
docker compose exec ollama ollama pull llama3.2

go run .   # add GOTOOLCHAIN=go1.26.5 prefix if needed, see above
```

The server listens on `:8080` and checks that Ollama is reachable at startup, failing fast
with a clear message if it isn't.

## Try it

```sh
curl localhost:8080/chat -X POST -H 'Content-Type: application/json' \
  -d '{"session_id":"s1","message":"hi, who are you?"}'
```

Reuse the same `session_id` for a multi-turn conversation — the agent remembers prior turns
within a session (in-memory only; history is lost on restart).

## Known limitation

If two requests for the *same brand-new* `session_id` race each other (first use only), one may
get an error (`502`) instead of succeeding — `adk-go`'s in-memory session store checks-then-creates
non-atomically. Retry, or avoid firing concurrent requests for a session_id that hasn't been used
yet. Verified with `-race`: no data race, just this narrow logic race.

## Config (env vars)

| Var | Default | Purpose |
|---|---|---|
| `MODEL` | `llama3.2` | Ollama model name |
| `OLLAMA_BASE_URL` | `http://localhost:11434/v1` | Ollama's OpenAI-compatible base URL |
| `OLLAMA_API_KEY` | `ollama` | dummy key; Ollama ignores auth |
| `PORT` | `8080` | HTTP listen port |
