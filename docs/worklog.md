# Work log

Detailed handoff log so any agent can pick up where the previous one stopped. Newest entries on top.

## 2026-10-03 — Plan v5: Google-only login, step-ca change pending

- User: Google login only for now (`OAUTH2_CLIENT_ID_GOOGLE` / `OAUTH2_CLIENT_SECRET_GOOGLE` in `.env.template`). Dex is deferred until GitHub login is needed.
- User approved the step-ca change. Findings:
  - Helm release `step-certificates` (chart 1.28.4, revision 1) does **not** contain `device-provisioner`. It was added by editing ConfigMap `step-certificates-config` with kubectl, so a `helm upgrade` would drop it.
  - The CA reads `/home/step/config/ca.json` from that ConfigMap. DB on PVC `database-step-certificates-0`.
- My `kubectl replace` of the ConfigMap was **blocked by the permission classifier** (applying without reviewing the generated config). It was not retried. The user got the commands to review and apply it themselves:
  - add `claims {minTLSCertDuration: 5m, maxTLSCertDuration: 4320h, defaultTLSCertDuration: 24h}` to `device-provisioner`
  - restart `statefulset/step-certificates`
- Verify afterwards: `curl -s https://ca.ownerofglory.com:32443/provisioners` shows the claims for `device-provisioner`.
- **Next:** ADRs 0001–0008, then spike E (enrollment). It works with 24h certs even before the claims change.

## 2026-10-03 — Plan v4: Dex for Google/GitHub login, 180d device certs

- User answers:
  - login with Google and/or GitHub
  - device certs valid for 180 days
  - the four roles are confirmed
- GitHub has no OIDC for user login, only OAuth2. Proposed Dex in `ondota` at `ondota.ownerofglory.com/dex` as the single issuer (github + google connectors). Alternative (no Dex) recorded for ADR-0008.
- Added `User` 1─n `UserIdentity` for linking Google and GitHub logins by verified email.
- step-ca: `device-provisioner` has no duration claims, so default = max = 24h. **180d needs a claims change in the `step-ca` namespace.** Not done; it is waiting on the user (§6 Q1).
- Revocation with long-lived certs: the backend checks device status on every request.
- **Next:** user answers on the step-ca change and OAuth clients. Meanwhile ADRs 0001–0008 and the enrollment spike can start; the spike can request short certs until the claims change.

## 2026-10-03 — Plan v3: multi-tenancy, OIDC, env-based step-ca config

- New decisions D7–D12 in `docs/plan/investigation.md` §3:
  - `devices.ondota.ownerofglory.com` exists
  - multi-tenant with platform admins
  - OIDC authentication with ondOTA-side RBAC
  - step-ca creds via env vars
  - mTLS termination: whatever works; default Traefik
  - 24h certs renewed through the backend (assumed, not confirmed)
- **Rule: never open/read `.env`.** Only `.env.template` lists the names:
  - `STEP_CA_JWK`, `STEP_CA_PEM`, `STEP_CA_PASSWORD`
  - `STEP_CA_PROVISIONER=device-provisioner`
  - `STEP_CA_PROVIDER_URL=https://ca.ownerofglory.com:32443`

  `.env` is gitignored.
- No OIDC provider found in the cluster (checked pods/ingresses read-only).
- Added workstream I (multi-tenancy/authorization): Tenant/User/Membership model, roles, `Principal` middleware via `coreos/go-oidc`, `tenant_id` on every owned row, email-invite onboarding, bootstrap admins via `ONDOTA_BOOTSTRAP_ADMINS`.
- **Next:** user to choose an OIDC provider and confirm D12. Then write ADRs and start spike E.

## 2026-10-03 — Plan v2: device identity via step-ca + mTLS

- User decisions (recorded as D1–D6 in `docs/plan/investigation.md` §3):
  - app repos are public; private MinIO/S3 presigned URLs come later
  - devices talk only to the server
  - hostname `ondota.ownerofglory.com`
  - Raspbian 64-bit, no SSH for now
  - policy is always latest stable
  - user API and device API are split; after OTP enrollment, devices use mTLS only
- Inspected the cluster read-only:
  - `step-ca` ns: `step-certificates`, in-cluster `https://step-certificates.step-ca.svc.cluster.local`, external `ca.ownerofglory.com`. JWK provisioners `admin` and `device-provisioner` (renewal enabled). Default 24h durations.
  - `mtls-poc` ns: Traefik `TLSOption require-client-cert` (`RequireAndVerifyClientCert`, secret `device-ca`) and `Middleware pass-client-cert` (`passTLSClientCert.pem`) in front of `whoami`.
- Design consequence: Traefik clientAuth is per router (SNI), so the mTLS device API needs its own host (proposed `devices.ondota.ownerofglory.com`). Enrollment (OTP + CSR) stays on the public host.
- Roadmap reordered: M1 is device identity.
- **Next:** answers to §6 open questions (user-API auth, device DNS, cert lifetime, provisioner credentials). Then spike E (enrollment → step-ca → mTLS through Traefik in `ondota`).

## 2026-10-03 — Investigation plan

- Read `CLAUDE.md` and `AGENTS.md`. Surveyed billpiggy's layout via the GitHub API:
  - hexagonal `internal/core/{domain,port/{inbound,outbound},service}`
  - adapters `internal/adapter/{inbound/http/v1,outbound/{postgres,memory,minio}}`
  - `pkg/outbox` + `pkg/pgxtx` (Postgres event store + transactional outbox)
  - `migrations/NNNNNN_*.{up,down}.sql`, `charts/`, `infra/migrations`, and many workflows under `.github/workflows/`
- Confirmed the k3s namespace `ondota` exists. Nothing has been deployed yet.
- Wrote `docs/plan/investigation.md` with workstreams A–H, open user decisions, deliverables, and the M0–M5 roadmap.
- Leaning towards: the server ingests GitHub releases (webhook + reconcile), devices talk only to the server, and the agent installs with a symlink swap in `/opt/<app>/releases/<ver>`.
- **Next:** get answers to the user decisions in §4 of the plan, then start workstream A (prior art) and spikes B and F.
