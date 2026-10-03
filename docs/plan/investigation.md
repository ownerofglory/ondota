# ondOTA — Investigation Plan

Status: draft v5 · 2026-10-03

## 1. Goal

Replace the manual `scp` workflow for Raspberry Pi apps with a pull-based update system:

```
git tag v1.2.0 → GitHub Release (asset + checksums.txt)
             → ondota-server (knows releases, devices, desired state)
             → ondota-agent on device (check → download → verify → install atomically → restart → report)
```

**MVP scope:** one user, a few Pis, public GitHub Releases as the only artifact source, systemd services as the only install target.

**Later (design for it, don't build it yet):** private object storage (MinIO/S3 with presigned URLs), Debian packages, custom ECU/HPC formats, staged rollouts, a Rust agent, standards like Uptane/AUTOSAR.

## 2. Constraints (from CLAUDE.md)

- Go backend, hexagonal architecture, layout mirrors [billpiggy](https://github.com/ownerofglory/billpiggy) (`cmd/`, `internal/core/{domain,port,service}`, `internal/adapter/{inbound,outbound}`, `pkg/`, `migrations/`, `charts/`, `.github/workflows/`).
- Event-driven, but no Kafka. Billpiggy's PostgreSQL event store + `pkg/outbox` is the default candidate.
- No Docker locally. Dev infra (Postgres etc.) runs in the k3s namespace `ondota` (namespace exists). Nothing destructive outside it.
- Server and agent live in this repo. Unit and integration tests, CI/CD for both, godoc on exported code.
- Don't overengineer. Use a solid OSS component when one fits.
- Keep `CHANGELOG.md` (short entries) and `docs/worklog.md` (detailed agent handoff log).

## 3. Decisions taken

| # | Decision |
|---|---|
| D1 | App repos are **public** for now. Private artifacts will come later through MinIO/S3 presigned URLs, so the download step returns an "artifact URL" abstraction from day one. |
| D2 | Devices talk **only to the ondota server**. The server ingests GitHub releases. |
| D3 | Public hostname: `ondota.ownerofglory.com` (DNS A record exists). |
| D4 | Target OS: Raspberry Pi OS (Raspbian) 64-bit → agent builds for `linux/arm64` first. No SSH access for now, so the install spike (F) runs locally/in CI against a fake systemd, and the user runs it on the Pi manually. |
| D5 | Version policy: **always the latest stable release** (not draft, not pre-release). Pinning is a later override. |
| D6 | The API is split into a **user-facing API** and a **device-facing API**. After enrollment, devices authenticate **only with mTLS**, using certificates issued by the existing **step-ca**. |
| D7 | Device API host is `devices.ondota.ownerofglory.com` (DNS A record exists). |
| D8 | **Multi-tenant.** A tenant is an organisation that can see and update only its own devices. **Platform admins** create tenants, add users to them, and grant or limit their permissions. |
| D9 | Users log in with **Google** (OIDC) for now. The backend validates Google ID tokens directly (`coreos/go-oidc`, issuer `https://accounts.google.com`), so there is **no Dex for now**. Config: `OAUTH2_CLIENT_ID_GOOGLE`, `OAUTH2_CLIENT_SECRET_GOOGLE`. The issuer list is configurable, so GitHub can come later through Dex without changing the domain model (§5 I). ondOTA only *authenticates* users; tenants, memberships and roles live in its own DB. |
| D10 | step-ca credentials are provided through env vars (names in `.env.template`: `STEP_CA_JWK`, `STEP_CA_PEM`, `STEP_CA_PASSWORD`, `STEP_CA_PROVISIONER`, `STEP_CA_PROVIDER_URL`). **Never open or read `.env`.** Code and tooling refer to the variable names only. In k8s the same names come from a Secret. |
| D11 | mTLS termination: use whichever works better. Default to the proven `mtls-poc` pattern (Traefik terminates TLS and forwards the cert). |
| D12 | Device certs live **180 days** (`4320h`) and are renewed through the backend at about 2/3 of their lifetime. step-ca's `device-provisioner` gets `maxTLSCertDuration: 4320h` while the default stays 24h, so existing clients (mtls-poc) are unaffected. The backend requests `notAfter` explicitly. The user approved the change, which is **pending the user applying it** (see worklog). Revocation is enforced by the backend on every device request (Traefik only checks the chain). |
| D13 | Roles: `platform_admin`, and per tenant `admin` / `operator` / `viewer`, confirmed. |

## 4. Existing cluster assets (found 2026-10-03, read-only inspection)

- **step-ca** (namespace `step-ca`): StatefulSet `step-certificates`, reachable in-cluster at `https://step-certificates.step-ca.svc.cluster.local` and externally at `ca.ownerofglory.com`.
  - Provisioners: `admin` (JWK) and **`device-provisioner` (JWK, renewal enabled)**.
  - No explicit cert durations are set, so step-ca defaults apply (24h).
- **mtls-poc** (namespace `mtls-poc`): Traefik Ingress with:
  - `TLSOption require-client-cert`: `RequireAndVerifyClientCert`, CA from secret `device-ca`, TLS ≥ 1.2
  - `Middleware pass-client-cert`: `passTLSClientCert.pem=true`, which forwards the client cert to the backend in `X-Forwarded-Tls-Client-Cert`

  Backend: `whoami`.
- Ingress controller: Traefik (`traefik` IngressClass). cert-manager is installed.
- No OIDC provider runs in the cluster (no Keycloak/Authelia/Dex/Zitadel/Authentik found). Dex will be deployed in `ondota` (D9).
- step-ca provisioner `device-provisioner` sets no x509 duration claims, so the global defaults apply (default = max = 24h).
- `.env.template` points `STEP_CA_PROVIDER_URL` at the external NodePort (`https://ca.ownerofglory.com:32443`). In-cluster, the server could use `https://step-certificates.step-ca.svc.cluster.local` instead. It's configurable either way.

## 5. Workstreams

Each workstream ends in an ADR (`docs/adr/NNNN-*.md`) or a spike result.

### A. Prior art (half a day)
- **Eclipse hawkBit DDI API**: device polling contract and deployment/action model.
- **Mender, SWUpdate, RAUC, balena, UpdateHub**: rollback, device auth, artifact models.
- **TUF / Uptane**: which hooks to leave open now.
- **goreleaser**: `checksums.txt` format and asset naming, since app repos will follow this convention.

Output: ADR-0001.

### B. Release ingestion from GitHub
- GitHub `release` webhook (`published`/`released` actions), verified with `X-Hub-Signature-256`, plus a periodic reconcile poll with `ETag`. Public repos mean no token is strictly needed, but a PAT raises the rate limit from 60/h to 5000/h.
- Under D5, ignore draft and pre-release releases. `ReleasePublished` → event → desired state of subscribed devices is recomputed.
- Port `ArtifactSource` with the GitHub adapter now. A MinIO/S3 adapter later returns presigned URLs. The device always receives `{url, sha256, size}`, so moving to private storage needs no agent change.

Output: ADR-0002.

### C. Domain model and the manifest
- The device manifest says *which* apps it runs and *how* to install them (service, install_dir). The server decides *which version* (D5).
- Candidate aggregates: `Device` (status: `pending` → `enrolled` → `revoked`), `Application`, `Release` (+ `Artifact`), `Deployment`, `DeviceAppState`.
- Candidate events: `DeviceRegistered`, `DeviceEnrolled`, `DeviceCertRenewed`, `DeviceRevoked`, `DeviceManifestReported`, `ReleasePublished`, `DeploymentAssigned`, `UpdateStarted`, `UpdateSucceeded`, `UpdateFailed`, `UpdateRolledBack`.
- Agent-side ports: `Installer` / `PackageFormat` (tar.gz now; deb/custom later) and `ServiceManager` (systemd).

Output: `docs/architecture.md`.

### D. API split: user-facing vs. device-facing (D6)

| Surface | Host (proposal) | Auth | Endpoints (draft) |
|---|---|---|---|
| User API | `ondota.ownerofglory.com/api/v1` | OIDC bearer token (JWT) + ondOTA RBAC (§5 I) | Tenant-scoped: `/tenants/{tid}/devices` (`POST` register → OTP, `GET`, `DELETE` revoke), `/tenants/{tid}/applications`, `/tenants/{tid}/releases`, `/tenants/{tid}/devices/{id}/history`. Admin: `/admin/tenants`, `/admin/tenants/{tid}/members`, `/me` |
| Enrollment | `ondota.ownerofglory.com/enroll/v1` | server TLS + OTP | `POST /enroll` `{otp, csr}` → `{cert_chain, ca_bundle}` |
| Device API | `devices.ondota.ownerofglory.com/device/v1` | **mTLS only** | `PUT /manifest`, `GET /desired-state` (ETag), `POST /reports`, `POST /certificate/renew` `{csr}` |

**Why a separate device host:** Traefik applies `clientAuth` per TLS router, matched by SNI. It can't require a client cert on one path and not on another under the same host. Enrollment has no client cert yet, so it has to stay on a different router. The second host is decided in D7.

**Where mTLS terminates (spike):**
1. **Traefik terminates** (as in `mtls-poc`) and forwards the PEM in `X-Forwarded-Tls-Client-Cert`. The server parses the cert and maps it to the device ID from the CN/SAN, e.g. `URI:ondota:device:<uuid>`. The server must only accept that header from Traefik: NetworkPolicy, and the header is stripped on other routers.
2. **TLS passthrough** (`IngressRouteTCP` with `passthrough: true`) and the Go server verifies the client cert itself. This is end-to-end with no trusted header, but the server has to manage its own server cert (cert-manager).

Leaning towards (1), because it's proven in the cluster already. The verification logic sits behind an inbound "device identity" middleware either way.

The contract goes in `api/openapi.yaml`, split into user and device specs, language-neutral for a future Rust agent.

Output: ADR-0003.

### E. Device identity and security (D6)

Enrollment flow (proposal):

```
User ──POST /devices {name}──▶ server: Device(pending), OTP (random, ≥128 bit, hashed in DB, TTL ~15 min, single use)
User copies OTP to the Pi:  ondota-agent enroll --otp XXXX --server https://ondota.ownerofglory.com
Agent: generate ECDSA P-256 key (0600, never leaves device) + CSR (CN = device id)
Agent ──POST /enroll {otp, csr}──▶ server: verify+burn OTP, check CSR (key type, subject == device id, proof of possession)
server ──JWK OTT (device-provisioner, sub=device id, SANs pinned)──▶ step-ca /1.0/sign ──▶ cert chain
server ──▶ Agent: cert chain + CA bundle;  Device(enrolled), event DeviceEnrolled
Agent ──mTLS──▶ devices.ondota… from now on
```

The device belongs to exactly one tenant, decided when it is registered. The cert carries only the device ID (URI SAN `ondota:device:<uuid>`). The tenant is looked up in the DB on each request, so moving a device to another tenant needs no cert re-issue.

Questions to settle:
- **OTT signing.** Use the `go.step.sm/crypto` / `smallstep/certificates` client library, or a small hand-rolled JWT? Credentials come from the D10 env vars (`STEP_CA_JWK`, `STEP_CA_PASSWORD`, `STEP_CA_PROVISIONER`, `STEP_CA_PEM` for the root used to trust step-ca, `STEP_CA_PROVIDER_URL`). They are never read from `.env` by tooling.
- **Lifetime and renewal (D12: 180d).** step-ca's global default and max is 24h, and the provisioner sets no claims, so a 180d request would currently be rejected. Required change (by the user, or with their explicit go-ahead), one of:
  - add `claims` to `device-provisioner`: `defaultTLSCertDuration: 4320h`, `maxTLSCertDuration: 4320h` (plus a sane `minTLSCertDuration`)
  - a dedicated `ondota-devices` JWK provisioner carrying those claims, so other users of `device-provisioner` are unaffected

  The server also requests `notAfter` explicitly, so it doesn't depend on the default. The agent renews via the backend once 2/3 of the lifetime has passed (~120d) and retries daily. The original 24h options are kept below for reference:
  - Agent renews through the device API (`/certificate/renew` with a new CSR over mTLS). The server re-checks the device isn't revoked, then requests a new cert. Preferred, because revocation stays in our DB.
  - Agent calls step-ca `/renew` directly, which is simpler but bypasses our revocation.
  - Raise the provisioner's `defaultTLSCertDuration`, e.g. 7–30d. That changes step-ca config outside `ondota`, so it needs the user's explicit go-ahead.
- **Revocation.** Rely on short-lived certs plus a server-side device status check on every request (no CRL/OCSP). Revoke = mark the device revoked, and the cert dies at expiry.
- **Trust anchor.** The Traefik `TLSOption` needs the step-ca root/intermediate as a secret in `ondota` (like `device-ca` in `mtls-poc`).
- **Artifact authenticity.** SHA256 from `checksums.txt` only shows integrity. Evaluate cosign/minisign or GitHub artifact attestations for M4. Meanwhile the server-provided desired state (delivered over mTLS) is the trusted source of the expected hash.
- **Agent hardening.** Safe tar extraction (path traversal, symlinks, size limits), a dedicated user with narrow polkit rights for `systemctl` vs. running as root, key file permissions.

Output: ADR-0004, a threat model, and a spike that walks the whole OTP → CSR → step-ca → mTLS call through a Traefik route in `ondota`.

### F. Agent install mechanics (local spike, no SSH)
- Layout: `/opt/<app>/releases/<version>/` plus a `current` symlink switched atomically (create a temp symlink, then `rename(2)`). The unit uses `current`.
- Flow: download to temp → verify → extract → swap → `systemctl restart` → health check → report. On failure, swap back and restart.
- Retention (keep the last N releases), disk checks, crash recovery mid-swap.
- `ServiceManager`: `systemctl` exec vs. `coreos/go-systemd` D-Bus. Faked in tests. A real-Pi test happens when the user runs it.
- Agent self-update, and packaging through goreleaser (`linux/arm64`, then `linux/arm`) + `.deb`/install script + systemd unit.

Output: PoC and ADR-0005.

### G. Server infrastructure and event plumbing
- Reuse billpiggy's `pkg/outbox`, `pkg/pgxtx`, `pkg/health`, `pkg/metrics`, migration job, Helm chart, and workflows.
- Decide between full event sourcing and state tables + outbox (the update history is naturally an event log).
- `ondota` namespace:
  - Postgres
  - two Traefik routers: public/user + enrollment, and mTLS device
  - `TLSOption` + `passTLSClientCert` middleware
  - cert-manager for the public server cert
  - Secrets: step-ca provisioner key and webhook secret
- Integration tests: Postgres in `ondota` through `kubectl port-forward` locally, GitHub Actions `services: postgres` in CI. Use step-ca's `authority` package in tests, or a test CA, so CI doesn't depend on the cluster's CA.

Output: ADR-0006.

### I. Multi-tenancy and authorization (D8, D9)
- **Model:**
  - `Tenant`
  - `User` with 1─n `UserIdentity` (OIDC `issuer` + `sub` per Google/GitHub login; email for display and invites)
  - `Membership` (user × tenant × role)
  - a global `platform_admin` flag

  Every tenant-owned row (`devices`, `applications`, `deployments`, `device_app_state`, OTPs, events) carries `tenant_id`.
- **Roles (start small):**
  - `platform_admin`: creates tenants, manages any membership
  - tenant `admin`: manages its own members, devices and apps
  - `operator`: registers/revokes devices, manages apps
  - `viewer`: read-only

  Roles map to a fixed permission set in code. Per-user grants beyond roles are a later step if needed.
- **Login (Google now, more later):**
  - The user API accepts Google ID tokens as bearer tokens. The backend validates issuer, audience (`OAUTH2_CLIENT_ID_GOOGLE`), signature (JWKS) and `email_verified`.
  - The CLI/UI gets tokens through the authorization-code + PKCE flow, using the client secret where Google requires it.
  - Trusted issuers are a config list. Adding GitHub later means deploying Dex in `ondota` (Dex at `ondota.ownerofglory.com/dex`, github + google connectors) and adding its issuer. Nothing changes in the domain.
- **Identity linking:** one person can log in via Google *and* GitHub, giving two different `sub`s. Model this as `User` 1─n `UserIdentity(issuer, connector, sub, email, email_verified)`. Memberships hang off `User`. A new identity links to an existing user only through a verified email. Check that Dex's GitHub connector only exposes verified emails.
- **Enforcement:** an inbound auth middleware verifies the OIDC JWT (`coreos/go-oidc`: issuer, audience, JWKS) and resolves user + memberships into a `Principal` in the context. Application services check `Principal.Can(tenantID, perm)`. Repositories always take `tenantID`. Decide whether to add Postgres row-level security as defense in depth or rely on tests.
- **Onboarding:** an admin adds a user to a tenant by email *before* that user's first login. The pending membership binds to the OIDC identity on first login, and only if the email is verified. Platform admins are bootstrapped from config (`ONDOTA_BOOTSTRAP_ADMINS`, a list of emails or subs).
- **Shared GitHub repos:** releases are ingested once per repo (globally). Tenant `Application`s reference a repo. The webhook secret lives per application/repo registration.
- **Devices:** the device API resolves cert → device → tenant. A device never sees another tenant's data.
- The UI is out of scope for now. The API + OpenAPI is enough, and a CLI can do the OIDC device-code flow.

Output: ADR-0008 and a tenant-isolation test suite (every repository method has a cross-tenant negative test).

### H. Repo layout and CI/CD
- One module containing:
  - `cmd/ondota-server`
  - `cmd/ondota-agent`
  - `internal/agent/...`
  - shared wire types from OpenAPI (oapi-codegen) or a small `pkg/api`
- CI: PR checks; server image, Helm, deploy; agent goreleaser on tag. Plus a template app repo (goreleaser + systemd unit).

Output: ADR-0007.

## 6. Open questions for the user
None blocking. Pending action: the user applies the step-ca claims change (D12).

## 7. Investigation deliverables
- `docs/adr/0001…0008`
- `docs/architecture.md`
- `api/openapi-user.yaml`, `api/openapi-device.yaml` (drafts)
- Spikes: B (GitHub ingestion), E (enrollment + mTLS end-to-end in `ondota`), F (atomic install, local)
- `CHANGELOG.md`, `docs/worklog.md`

## 8. Tentative roadmap after investigation
- **M0 Skeleton:** layout, CI, Helm, Postgres in `ondota`, health endpoints, both Traefik routers (`ondota.` + `devices.ondota.`), config from env (D10).
- **M1 Identity:** Google login (Dex/GitHub later), user identity linking, tenants, memberships and roles (platform admin), tenant-scoped device registration + OTP, enrollment through step-ca, mTLS device API, cert renewal, revocation.
- **M2 Releases and desired state:** GitHub webhook + reconcile, latest-stable policy, manifest registration, `GET /desired-state`.
- **M3 Agent:** enroll, poll, download, verify, atomic install, restart, rollback, report. Packaged for arm64. The user installs it on the Pi, replacing `scp`.
- **M4 History and hardening:** update history API, simple CLI (`ondotactl`), artifact signatures, agent self-update, metrics.
- **M5 Extensibility:** MinIO/S3 `ArtifactSource` with presigned URLs, pinning/channels, staged rollouts.

M1 comes before the agent work because every device call depends on mTLS. The agent install logic (F) can still be developed in parallel because it has no server dependency.
