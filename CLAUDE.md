# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

Minimal learning AI agent: [gin-gonic/gin](https://github.com/gin-gonic/gin) HTTP layer,
[google/adk-go](https://github.com/google/adk-go) agent framework, talking to any of several
preconfigured hosted OpenAI-compatible Chat Completions providers (see Config). Single `/chat`
endpoint, in-memory session history, no user auth, no persistence, no tool-calling.

## Prerequisites

See README.md for full setup (API key). One thing to know for every Go command in this repo:
`go.mod` requires Go >= 1.27.0. If local `go version` is older and `go env GOTOOLCHAIN` is
`local`, prefix commands with `GOTOOLCHAIN=go1.27.0` — otherwise `go run`/`go build`/`go get`
fail with `go.mod requires go >= 1.27.0`.

If `go build`/`go vet` instead fail with `operation not permitted` writing to the module
cache, that's a Claude Code sandbox filesystem restriction, not the toolchain issue above —
rerun with the sandbox disabled rather than adjusting `GOTOOLCHAIN`.

## Commands

```sh
go run ./cmd/the-agent          # run from repo root; add GOTOOLCHAIN=go1.27.0 prefix if needed
go build ./...
go vet ./...
```

Table-driven tests cover `internal/config` (`config.Load`) and `cmd/the-agent`
(`resolveModel`); run with `GOTOOLCHAIN=go1.27.0 go test ./...`. There is no lint config
beyond `go vet`.

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
- `outbound.AgentEnginePort` has two methods: `RunTurn` (one agent turn) and `Ping`
  (reachability check). `Ping` reuses the same `*openai.Client` the model uses (e.g.
  `client.Models.List`) rather than a separate hand-rolled HTTP request — don't duplicate the
  auth/base-URL wiring. `Ping` is currently unused by `main.go` — reachability is proven at
  startup by the model-discovery call instead (see Config) — but stays on the port/Adapter for
  future use (e.g. a deep health-check endpoint). Don't remove it as dead code without checking
  first whether something else has started calling it.
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

## CI

`.github/workflows/ci.yml`'s `build-test`/`lint` jobs use `actions/setup-go@v7` +
`go-version-file: go.mod`, which already resolves and downloads the exact `go.mod`
version (1.27.0) regardless of the runner's preinstalled Go. If a Go-version CI
failure gets reported, adding an explicit `go-version: 'X.Y'` pin or downgrading to
`actions/setup-go@v5` is not the fix — both were suggested and rejected this project.

`govulncheck` and `gosec` jobs deliberately have no `setup-go` step:
`govulncheck-action` provisions its own Go via its own `go-version-file` input, and
`gosec` runs as a Docker container that ignores the host's Go entirely.

Known gotcha: `gosec`'s container reports `GOTOOLCHAIN=local` regardless of host
env — confirmed even with no `-e "GOTOOLCHAIN"` in the `docker run` invocation — so
its bundled Go (1.26.5 in the pinned image) can't auto-upgrade to satisfy go.mod's
`go >= 1.27.0`, failing with `go.mod requires go >= 1.27.0 (running go 1.26.5;
GOTOOLCHAIN=local)`. Fix: step-level `env: GOTOOLCHAIN: auto` on the `gosec` step
(docker `-e` overrides image env). If this resurfaces, check that override is still
present before re-diagnosing from scratch; the durable fix is bumping to a gosec
image built against Go >= go.mod's version and dropping the override.

Third-party actions (`golangci-lint-action`, `gosec`) must stay pinned by commit
SHA, not a bare version tag — the commit security review gate flags a bare tag on
either as HIGH severity (supply-chain-pinning-regression).

## Config

`config/config.yaml` defines a named `providers` map, loaded via
[spf13/viper](https://github.com/spf13/viper) (`internal/config`). Each provider entry has a
`model` **preference list** (not a fixed model — see below), `base_url`, and `api_key_env`
(the name of the env var holding that provider's key — never a raw key in the file).

`config.Load` (`internal/config/config.go`) returns `[]Candidate` — it never resolves a final
model name itself:
- `provider`/`PROVIDER` unset (the normal case, no default): every provider whose
  `api_key_env` is actually set in the environment becomes a candidate, in alphabetical order
  by provider name (`knownProviders` sorts — config.yaml's own key order isn't preserved, since
  viper hands the `providers` map back as an unordered `map[string]any`).
- `provider`/`PROVIDER` set: only that provider is used (`model`/`MODEL`, if also set, becomes
  that candidate's `ModelOverride`). Fails fast if that provider is unknown or its key env var
  is unset — no falling back to auto-selection.

`cmd/the-agent/main.go`'s `selectEngine` tries each candidate in turn: it calls
`openaicompatoutboundadapter.ListModels` (`GET {base_url}/models`) to fetch that provider's
*live* model list — a successful call also proves reachability/auth, which is why there's no
separate `Ping` here (see above) — then `resolveModel` picks which model to actually use:
`ModelOverride` if set (unvalidated — the operator asked for it by name), else the first of
`ModelPreferences` that's actually present in the live list. If neither matches, that
candidate fails outright (naming the live models in the error) rather than guessing — a
provider's live list can include non-chat models (Groq's includes `whisper-large-v3`,
`llama-guard-3-8b`, etc. alongside chat models) that config.yaml's preferences were never
meant to match, so picking index 0 of the live list would risk silently running a model that
can't do chat completions at all.

`config.Load` fails fast if: `providers` is empty or missing, an explicit `provider` names an
entry not in the map, an entry's `model` list is empty, or (explicit-selection path only) its
`api_key_env` is unset. In the auto-selection path a provider with no key set is just skipped,
not fatal — `Load` only fails if *none* have a key set. `config/config.yaml` is effectively
required — there's no hardcoded provider map to fall back to if it's missing.

Known gotcha, and why the "fail rather than guess" rule above matters: the reachability check
is only as strict as the provider's `/models` endpoint makes it — OpenRouter's returns 200 for
any bearer token, valid or not, so an invalid `OPENROUTER_API_KEY` can still get auto-selected
as "reachable" and only fail once a real chat request is made. OpenAI and Groq do reject
invalid keys at this endpoint.

`go run ./cmd/the-agent` must run from the repo root so `config/config.yaml` resolves via
viper's relative `config` search path. Startup now costs one model-list round-trip per
candidate tried (bounded at 5s each) rather than always exactly one.

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
