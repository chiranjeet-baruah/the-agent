# the-agent

Minimal learning AI agent: [gin-gonic/gin](https://github.com/gin-gonic/gin) HTTP layer,
[google/adk-go](https://github.com/google/adk-go) agent framework, [Docker Model
Runner](https://www.docker.com/blog/run-llms-locally/) as the local model backend.

## Prerequisites

- Docker Desktop with Model Runner enabled: `docker desktop enable model-runner --tcp=12434`
  (note the `=`; `--tcp 12434` with a space is silently accepted but doesn't reliably apply —
  verified live). Auto-enabled by default on Apple Silicon in Docker Desktop 4.40+. Unlike a
  containerized model server, Model Runner executes the inference engine as a host process, so
  it gets native Metal acceleration on Apple Silicon.
- Go >= 1.26.5 — `google.golang.org/adk/v2` requires it. If your local `go version` is older
  and `go env GOTOOLCHAIN` is `local`, prefix Go commands with `GOTOOLCHAIN=go1.26.5` (Go will
  download that toolchain on first use). If `GOTOOLCHAIN` is `auto` (Go's default), no prefix
  is needed.

## Run

```sh
docker model pull ai/llama3.2

go run ./cmd/the-agent   # run from repo root — see Config below; add GOTOOLCHAIN=go1.26.5 prefix if needed
```

The server listens on `:8080` and checks that Docker Model Runner is reachable at startup,
failing fast with a clear message if it isn't.

Note: Model Runner implements the OpenAI **Chat Completions** API (`/chat/completions`), not
the newer Responses API — that's why the outbound adapter below has its own small `model.LLM`
implementation instead of reusing adk-go's `model/openaimodel` package, which is hardcoded to
the Responses API.

## Architecture

Hexagonal (ports & adapters), per [this
spec](https://prabogo.com/docs/architecture.html):

```
cmd/the-agent/               composition root: wires everything, starts the HTTP server
internal/domain/             business logic; depends only on port interfaces, no frameworks
internal/port/inbound/       ChatPort — the use case inbound adapters drive
internal/port/outbound/      AgentEnginePort — what the domain needs from an LLM engine
internal/model/              plain request/response DTOs
internal/adapter/inbound/    gin_inbound_adapter — HTTP → ChatPort
internal/adapter/outbound/   dockermodelrunner_outbound_adapter — AgentEnginePort → Docker
                              Model Runner (adk-go LlmAgent/Runner + the Chat Completions
                              model.LLM implementation live here)
```

The domain has exactly one implementation per port (no second inbound or outbound adapter is
planned) — that's a deliberate trade of extra ceremony for strict adherence to the spec above,
not a claim that this project needed the abstraction on its own merits.

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

## Config

Defaults live in [`config/config.yaml`](config/config.yaml), loaded via
[spf13/viper](https://github.com/spf13/viper) (`internal/config`). Any key can be
overridden with an environment variable of the same name — useful for one-off
runs without editing the file.

| Key / env var | Default | Purpose |
|---|---|---|
| `model` / `MODEL` | `ai/llama3.2` | Docker Model Runner model name |
| `model_runner_base_url` / `MODEL_RUNNER_BASE_URL` | `http://localhost:12434/engines/v1` | Model Runner's OpenAI-compatible base URL |
| `port` / `PORT` | `8080` | HTTP listen port |

`go run ./cmd/the-agent` must run from the repo root so `config/config.yaml`
resolves via viper's relative `config` search path; the file is optional — if
missing, hardcoded defaults (matching the checked-in file) apply.
