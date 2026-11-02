# AGENTS.md — go-nexus

## 1. What this repo is

The Go client for the Nexus API (`aidenappl/nexus`). It owns request/response types, error
decoding and the resuming SSE stream. It does not own behaviour: the server decides everything,
and the contract is `docs/spec.md` (NXS-1) in the nexus repo.

## 2. Stack & dependencies

Go 1.25+, standard library only. Keep it that way: relay, dev and sites-api all import it.

## 3. Project structure

```
nexus.go    Client (New, WithURL, WithHTTPClient), Error/IsCode, do/text helpers, Footer
types.go    response types mirroring src/structs.rs in nexus
api.go      endpoint methods
stream.go   Stream: Last-Event-ID resume, skip seq <= last, reconnect on end/lagged/network
nexus_test.go  httptest unit tests + TestLive (env-gated, against a real server)
```

## 4. Running, building & testing

`dev test` runs the unit tests. For the live test, run a local nexus (`dev run` in the nexus
repo), create a user (`nexus admin create-user`), a namespace, then:
`NEXUS_TEST_URL=http://127.0.0.1:8000 NEXUS_TEST_TOKEN=nxp_… NEXUS_TEST_NS=… go test -run TestLive`.

## 5. How code is written here

- One method per endpoint, named for the action (`Claim`, `ReviseThread`), taking `ctx` first.
- Request structs use `omitempty` so unset fields carry over on revisions; pointers where an
  empty string is a meaningful value (`Revise.Status`).
- Markdown variants (`*Markdown`) exist for anything a hook puts in an agent's context; the
  server appends the untrusted-content footer.
- **Read the handler in the nexus repo before adding a method** — never infer a request shape.

## 6. Domain & architecture

Threads are immutable revision chains; writes send `base_rev` and may fail with
`stale_revision` (`IsCode(err, "stale_revision")`). `nxa_` tokens are minted from `nxp_`
tokens (`MintAgentToken`) and revoke themselves at session end (`RevokeSelf`).

## 7. Ecosystem & related repos

`nexus` (the API), `dev` (`dev nexus …`), `relay` (relayd), `sites-api` (connector).

## 8. Operations

A library; no deployment. Consumers pin a tag.

## 9. Rules & guardrails

- No third-party dependencies.
- Never log tokens. Never add a method that sends `agent`/`harness` in a body: the server
  attests them.
- Anything rendering Nexus text into an agent's context must keep `Footer` (spec §11.2).

## 10. Verification

```bash
gofmt -l . && go vet ./... && go test ./...
```

## 11. Keeping this file updated

Any change to the API surface, types or stream behaviour updates this file and `README.md` in
the same change, and follows the nexus spec.
