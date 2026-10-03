# ADR-0007: Repository layout and CI/CD

- Status: accepted
- Date: 2026-10-03

## Decision
One Go module (`github.com/ownerofglory/ondota`), following billpiggy's hexagonal layout:

```
cmd/ondota-server/          server main
cmd/ondota-agent/           agent main (enroll, run, version)
config/                     server config from env
internal/core/{domain,port/{inbound,outbound},service}   server core
internal/adapter/inbound/http/{user,device,enroll,hooks}/v1
internal/adapter/outbound/{postgres,memory,stepca,github}
internal/agent/{core,adapter}   agent, hexagonal as well; never imports server internals
pkg/                        reusable helpers (outbox, pgxtx, health, metrics)
api/openapi-user.yaml, api/openapi-device.yaml
migrations/                 golang-migrate SQL
charts/ondota/              server Helm chart
deploy/agent/               systemd unit + install script
spikes/                     throwaway spikes (not built in CI)
docs/{adr,plan}/, CHANGELOG.md, docs/worklog.md
```

- **Wire types:** the device contract is defined in OpenAPI. Go types for the agent and the server come from `oapi-codegen` (types only, no generated server), so a Rust agent can generate from the same spec.
- **CI (GitHub Actions, adapted from billpiggy):**
  - `pull-request-checks`: `go vet`, golangci-lint, unit tests, integration tests with a Postgres service, OpenAPI lint.
  - `docker-*`: server image to GHCR on `main` and on tags.
  - `helm-*` + deploy: Helm upgrade into `ondota` (kubeconfig secret scoped to namespace `ondota`).
  - `agent-release`: goreleaser on tag `agent/vX.Y.Z` → `linux/arm64`, `linux/arm` tar.gz + `checksums.txt` + `.deb`. The agent can then update itself through ondOTA (dogfooding).
- A template app repo (goreleaser config + systemd unit) documents the app convention.

## Consequences
- One module keeps refactors cheap. The agent/server boundary is enforced by import rules (a depguard lint), not by separate modules.
