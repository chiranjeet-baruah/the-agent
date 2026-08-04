# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

Minimal learning AI agent: [gin-gonic/gin](https://github.com/gin-gonic/gin) HTTP layer,
[google/adk-go](https://github.com/google/adk-go) agent framework, talking to any
OpenAI-compatible Chat Completions backend — defaults to [Docker Model
Runner](https://www.docker.com/blog/run-llms-locally/) for local dev. Single `/chat`
endpoint, in-memory session history, no auth, no persistence, no tool-calling.

## Prerequisites

See README.md for full setup (Docker Model Runner, model pull). One thing to know for every
Go command in this repo: `go.mod` requires Go >= 1.26.5. If local `go version` is older and
`go env GOTOOLCHAIN` is `local`, prefix commands with `GOTOOLCHAIN=go1.26.5` — otherwise
`go run`/`go build`/`go get` fail with `go.mod requires go >= 1.26.5`.

## Commands

```sh
go run ./cmd/the-agent          # run from repo root; add GOTOOLCHAIN=go1.26.5 prefix if needed
go build ./...
go vet ./...
```

No test suite exists yet. There is no lint config beyond `go vet`.

Manual smoke test:

```sh
curl localhost:8080/chat -X POST -H 'Content-Type: application/json' \
  -d '{"session_id":"s1","message":"hi, who are you?"}'
curl localhost:8080/health
```

Reuse the same `session_id` for multi-turn conversation — history is per-session, in-memory,
lost on restart.

## Architecture

Strict hexagonal (ports & adapters), per [this
spec](https://prabogo.com/docs/architecture.html). One implementation per port only — no
second inbound or outbound adapter is planned; this is a deliberate trade of extra ceremony
for adherence to the spec, not a claim the project needed the abstraction on its own merits.

```
cmd/the-agent/               composition root: wires everything, starts the HTTP server
internal/domain/             business logic; depends only on port interfaces, no frameworks
internal/port/inbound/       ChatPort — the use case inbound adapters drive
internal/port/outbound/      AgentEnginePort — what the domain needs from an LLM engine
internal/model/              plain request/response DTOs (json tags, no logic)
internal/adapter/inbound/gin_inbound_adapter/         HTTP → ChatPort
internal/adapter/outbound/openaicompat_outbound_adapter/
                              AgentEnginePort → any OpenAI-compatible Chat Completions
                              backend (adk-go LlmAgent/Runner + a hand-rolled Chat
                              Completions model.LLM implementation)
internal/config/             viper-based config loader
```

Dependency direction: `cmd` → `adapter` → `domain` → `port`. Adapters depend on the domain
via ports; the domain never imports gin, adk-go, or genai types directly.

Adapter package/directory names are deliberately snake_case (`gin_inbound_adapter`,
`openaicompat_outbound_adapter`) to name-match the port + technology they implement — not
idiomatic Go, but intentional. Don't rename to camelCase.

- `domain.ChatDomain` implements `inbound.ChatPort`; it just applies a 60s timeout
  (`runTimeout` in `internal/domain/chat.go`) and delegates to `outbound.AgentEnginePort`.
  There's no per-user auth — a single hardcoded `localUserID` stands in for a real user system.
- `outbound.AgentEnginePort` has two methods: `RunTurn` (one agent turn) and `Ping` (startup
  reachability check). `cmd/the-agent/main.go` calls `Ping` at startup and fails fast with a
  clear message if the configured backend isn't reachable.
- The outbound adapter wraps an adk-go `runner.NewInMemory` + `llmagent.New`. Session state
  lives entirely inside adk-go's in-memory session store (not this repo's code).

### Why the custom `llmModel` (internal/adapter/outbound/openaicompat_outbound_adapter/llm.go)

Not every OpenAI-compatible backend implements the newer Responses API — Docker Model
Runner, for example, only exposes the OpenAI **Chat Completions** API (`/chat/completions`).
adk-go's own `model/openaimodel` package is hardcoded to the Responses API, so it can't be
reused here. `llmModel` implements adk-go's `model.LLM` interface directly against
`openai-go/v3`'s Chat Completions client instead.

Known gaps in this hand-rolled model, both deliberate fail-fast rather than silent:
- No streaming — `GenerateContent` errors immediately if `stream` is true.
- No tool-call conversion — `llmagent.New` is never given `Tools`, and `GenerateContent`
  errors if `req.Tools` is non-empty, rather than silently never calling a tool.

### Known limitation: first-use session race

If two requests for the *same brand-new* `session_id` race each other (first use only), one
may get a `502` instead of succeeding — adk-go's in-memory session store checks-then-creates
non-atomically. Verified with `-race`: no data race, just this narrow logic race. Retry, or
avoid firing concurrent requests for a session_id that hasn't been used yet.

## Config

Defaults live in `config/config.yaml`, loaded via [spf13/viper](https://github.com/spf13/viper)
(`internal/config`). Any key can be overridden with an environment variable of the same name.

| Key / env var | Default | Purpose |
|---|---|---|
| `model` / `MODEL` | `ai/llama3.2` | Model name requested from the configured backend |
| `llm_base_url` / `LLM_BASE_URL` | `http://localhost:12434/engines/v1` | Base URL of the OpenAI-compatible Chat Completions backend |
| `port` / `PORT` | `8080` | HTTP listen port |

`go run ./cmd/the-agent` must run from the repo root so `config/config.yaml` resolves via
viper's relative `config` search path; the file is optional — if missing, hardcoded defaults
(matching the checked-in file) apply.

## Dependency pinning gotcha

adk-go v2.1.0 pins `go.opentelemetry.io/otel*` at specific versions (otel v1.43.0, otel/log
v0.19.0, otel/trace and otel/metric v1.43.0, otelhttp v0.68.0) and uses an internal API that
breaks under newer otel/log releases (e.g. v0.21.0 removes `log.Value`, `log.KeyValue`,
`log.StringValue`, `log.BoolValue`). A broad `go get -u ./...` will happily bump these past
what adk-go supports and break the build with `undefined: log.Value`-style errors. If that
happens, pin the otel family back down explicitly:

```sh
go get go.opentelemetry.io/otel@v1.43.0 go.opentelemetry.io/otel/log@v0.19.0 \
  go.opentelemetry.io/otel/trace@v1.43.0 go.opentelemetry.io/otel/metric@v1.43.0 \
  go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp@v0.68.0
```

Check adk-go's own `go.mod` (in the module cache) for the current required versions if this
recurs after an adk-go upgrade.
