# the-agent

[![CI](https://github.com/chiranjeet-baruah/the-agent/actions/workflows/ci.yml/badge.svg)](https://github.com/chiranjeet-baruah/the-agent/actions/workflows/ci.yml)
[![Go version](https://img.shields.io/github/go-mod/go-version/chiranjeet-baruah/the-agent)](go.mod)
[![License: MIT](https://img.shields.io/github/license/chiranjeet-baruah/the-agent)](LICENSE)

Minimal learning AI agent: [gin-gonic/gin](https://github.com/gin-gonic/gin) HTTP layer,
[google/adk-go](https://github.com/google/adk-go) agent framework, talking to any of several
preconfigured hosted OpenAI-compatible Chat Completions providers (see [Config](#config)).

**What it does:** one `/chat` endpoint, per-session conversation history, multi-provider
config with live model discovery.

**What it deliberately doesn't do:** no streaming, no tool-calling, no user auth, no
persistence (history is in-memory, lost on restart), no auto-guessing a model if none of the
configured preferences are live.

## Table of contents

- [Prerequisites](#prerequisites)
- [Quickstart](#quickstart)
- [API](#api)
- [Config](#config)
- [Architecture](#architecture)
- [Known limitation](#known-limitation)
- [License](#license)

## Prerequisites

- An API key for whichever provider is active (see [Config](#config)) — e.g. `GROQ_API_KEY`
  for the shipped `groq` provider. Never commit a real key to `config/config.yaml`; set it via
  the environment. `config.Load` fails fast at startup if no configured provider has its key
  set.
- Go >= 1.27.0 (pinned in `go.mod`). If your local `go version` is older and `go env
  GOTOOLCHAIN` is `local`, prefix Go commands with `GOTOOLCHAIN=go1.27.0` (Go downloads that
  toolchain on first use). If `GOTOOLCHAIN` is `auto` (Go's default), no prefix is needed.

## Quickstart

```sh
git clone git@github.com:chiranjeet-baruah/the-agent.git
cd the-agent

export GROQ_API_KEY=gsk_...
go run ./cmd/the-agent   # must run from repo root — see Config; add GOTOOLCHAIN=go1.27.0 prefix if needed
```

The server listens on `:8080` by default (configurable, see [Config](#config)) and checks
that the configured LLM backend is reachable at startup, failing fast with a clear message
if it isn't.

## API

### `POST /chat`

Request:

```sh
curl localhost:8080/chat -X POST -H 'Content-Type: application/json' \
  -d '{"session_id":"s1","message":"hi, who are you?"}'
```

Response:

```json
{"reply": "Hi! I'm a minimal AI agent — ask me anything."}
```

Reuse the same `session_id` for a multi-turn conversation — the agent remembers prior turns
within a session (in-memory only; history is lost on restart).

### `GET /health`

Returns `200` once the server is up — no dependency checks beyond startup.

## Config

[`config/config.yaml`](config/config.yaml) defines a named `providers` map — `groq` and
`openrouter` are enabled out of the box, with a commented-out `openai` entry as a template for
adding it back — loaded via [spf13/viper](https://github.com/spf13/viper) (`internal/config`).
Each entry lists a *preferred* model or two, a base URL, and the name of the env var holding
its API key:

```yaml
providers:
  groq:
    model: [llama-3.3-70b-versatile, llama-3.1-8b-instant]
    base_url: https://api.groq.com/openai/v1
    api_key_env: GROQ_API_KEY
```

`model:` is a preference order, not a fixed choice — at startup the app fetches that
provider's *live* model list and uses the first preference that's actually present there,
failing (not guessing) if none are. Providers rename/retire models over time, so a stale id
here just gets skipped rather than breaking startup, as long as at least one entry is still
valid.

| Key / env var | Default | Purpose |
| --- | --- | --- |
| `provider` / `PROVIDER` | none — auto-select | If set, use only this provider — fails fast if it's unknown or its key env var is unset, no silent fallback. If unset, try every provider whose key env var is set and use the first one that's actually reachable. |
| `model` / `MODEL` | (first live-matching preference) | Only applies when `PROVIDER` is set explicitly — used as-is (unvalidated) instead of matching against `model:`'s preferences |
| `port` / `PORT` | `8080` | HTTP listen port |

**No `PROVIDER` set (default):** just export whichever provider's key you have, and it's used.
Exporting more than one key at once is fine — auto-select tries each provider (alphabetically)
and uses the first that responds:

```sh
export GROQ_API_KEY=gsk_...
go run ./cmd/the-agent
```

**`PROVIDER` set explicitly:** only that provider is tried; startup fails fast if it's unknown
or its key env var is unset — it won't silently fall back to another provider.

```sh
export GROQ_API_KEY=gsk_...
export PROVIDER=groq
go run ./cmd/the-agent
```

Known gotcha: the model-discovery call hits `GET {base_url}/models`, and not every provider
validates the key there — OpenRouter's `/models` responds 200 even for an invalid key, so
auto-select can pick it over a provider that would've worked, and the bad key only surfaces on
the first real chat request. OpenAI and Groq do reject invalid keys at this endpoint.

`go run ./cmd/the-agent` must run from the repo root so `config/config.yaml` resolves via
viper's relative `config` search path — the file is required (there's no hardcoded provider
map to fall back to). Startup costs one model-list round-trip per candidate provider tried.

## Architecture

Hexagonal (ports & adapters), per [this
spec](https://prabogo.com/docs/architecture.html):

```text
cmd/the-agent/               composition root: wires everything, starts the HTTP server
internal/domain/             business logic; depends only on port interfaces, no frameworks
internal/port/inbound/       ChatPort — the use case inbound adapters drive
internal/port/outbound/      AgentEnginePort — what the domain needs from an LLM engine
internal/model/              plain request/response DTOs
internal/adapter/inbound/    gin_inbound_adapter — HTTP → ChatPort
internal/adapter/outbound/   openaicompat_outbound_adapter — AgentEnginePort → any
                              OpenAI-compatible Chat Completions backend (adk-go
                              LlmAgent/Runner + the Chat Completions model.LLM
                              implementation live here)
```

The domain has exactly one implementation per port (no second inbound or outbound adapter is
planned) — that's a deliberate trade of extra ceremony for strict adherence to the spec above,
not a claim that this project needed the abstraction on its own merits.

Not every OpenAI-compatible backend implements the newer Responses API — Docker Model Runner,
for example, only exposes the OpenAI **Chat Completions** API (`/chat/completions`). That's
why the outbound adapter above has its own small `model.LLM` implementation instead of reusing
adk-go's `model/openaimodel` package, which is hardcoded to the Responses API.

## Known limitation

If two requests for the *same brand-new* `session_id` race each other (first use only), one may
get an error (`502`) instead of succeeding — `adk-go`'s in-memory session store checks-then-creates
non-atomically. Retry, or avoid firing concurrent requests for a session_id that hasn't been used
yet. Verified with `-race`: no data race, just this narrow logic race.

## License

[MIT](LICENSE)
