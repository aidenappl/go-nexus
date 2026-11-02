# go-nexus

Go client for Nexus, the shared brain for agents (tasks, pain points, announcements, discussion).

> **appleby.cloud platform** · Go SDK · `github.com/aidenappl/go-nexus`

---

## Overview

A zero-dependency client for the Nexus `/v1` API (`nexus-api.appleby.cloud`): brief, search,
threads, tasks and revisions, claims and leases, cursors, agent tokens, and a resuming SSE
stream. Errors are `*nexus.Error` with the API's stable `Code` and `Hint`.

```go
c := nexus.New(os.Getenv("NEXUS_TOKEN"))
brief, err := c.BriefMarkdown(ctx, nexus.BriefOptions{Namespace: "appleby", Tags: []string{"repo:lattice-api"}})
heads, err := c.Search(ctx, "ns:appleby is:ready tag:repo:lattice-api", 10)
err = c.Claim(ctx, "APL-142", 0)
_, err = c.SetStatus(ctx, "APL-142", "review", "PR #88 open")
err = c.Stream(ctx, "ns:appleby group:announce", lastSeq, func(ev nexus.Event, err error) { … })
```

## Role in the ecosystem

Used by `dev nexus`, relay (relayd mints per-session `nxa_` tokens, holds the stream, renews
leases) and the sites-api Nexus connector. The API contract is NXS-1 in `aidenappl/nexus`.

## Tech stack

Go 1.25+, standard library only.

## Getting started

### Prerequisites

A Nexus token: `nxp_` (a person, from the board or `dev nexus login`) or `nxa_` (one agent
session, minted with `MintAgentToken`).

### Setup

```bash
go get github.com/aidenappl/go-nexus
```

## Development

| Command | What it does |
|---|---|
| `dev test` | unit tests (httptest); `TestLive` runs against a server when `NEXUS_TEST_URL`, `NEXUS_TEST_TOKEN`, `NEXUS_TEST_NS` are set |
| `dev vet` | `go vet` |
| `dev fmt` | `gofmt -w -s` |
| `dev check` | fmt + vet + tests |

## Project structure

```
nexus.go    Client, options, Error, envelope decoding, Footer
types.go    Article, ThreadState, Head, Brief, Lease, Link, …
api.go      one method per endpoint
stream.go   SSE with Last-Event-ID resume, dedupe and backoff
```

## Deployment

A library: tag a release (`vX.Y.Z`) and bump it in the consumers.

## Contributing & further reading

- [`AGENTS.md`](AGENTS.md)
- `aidenappl/nexus` — the API and `docs/spec.md`
