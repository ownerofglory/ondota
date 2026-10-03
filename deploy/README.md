# Deployment

Everything runs in the k3s namespace `ondota`. Secrets are created in the cluster and never committed.

## One-time cluster setup (done 2026-10-03)

| Resource | How | Notes |
|---|---|---|
| PostgreSQL | `kubectl apply -f deploy/postgres/postgres.yaml` | Secret `ondota-postgres` (`POSTGRES_USER=ondota`, `POSTGRES_DB=ondota`, random `POSTGRES_PASSWORD`). Extra DB `ondota_test` for integration tests. |
| App secret | `ondota-app` with `DATABASE_URL` (in-cluster service URL) | Later keys: step-ca and Google OAuth values, added from `.env` without printing them. |
| Device CA | Secret `device-ca` (`ca.crt` = step-ca root from `step-ca/step-certificates-certs`) | Traefik verifies device client certs against it. |
| CI identity | `kubectl apply -f deploy/ci/deployer.yaml` | ServiceAccount `ondota-deployer` with full rights **in `ondota` only**. |

## GitHub configuration for `.github/workflows/release.yaml`

Repository secrets:

| Secret | Value |
|---|---|
| `DOCKER_USER`, `DOCKER_TOKEN` | Docker Hub credentials (same as billpiggy). The image is `<DOCKER_USER>/ondota-server`. |
| `KUBERNETES_CLUSTER_SERVER_URL` | API server URL, as used for billpiggy |
| `KUBERNETES_CA_DATA` | `kubectl -n ondota get secret ondota-deployer-token -o jsonpath='{.data.ca\.crt}'` |
| `KUBERNETES_TOKEN` | `kubectl -n ondota get secret ondota-deployer-token -o jsonpath='{.data.token}' \| base64 -d` |

Create a GitHub environment `production`, optionally with required reviewers to gate migrations and deploys.

## Local development

```sh
make db-port-forward                       # terminal 1: Postgres on localhost:5432
PW=$(kubectl -n ondota get secret ondota-postgres -o jsonpath='{.data.POSTGRES_PASSWORD}' | base64 -d)
make test-integration TEST_DATABASE_URL="postgres://ondota:$PW@localhost:5432/ondota_test?sslmode=disable"
DATABASE_URL="postgres://ondota:$PW@localhost:5432/ondota?sslmode=disable" make run
```

## Spikes

`deploy/spike/mtls/` (echo service on `devices.ondota.ownerofglory.com`) must be deleted before the first chart deploy, because both use the same host:

```sh
kubectl delete -f deploy/spike/mtls/mtls.yaml   # keeps Secret device-ca
```
