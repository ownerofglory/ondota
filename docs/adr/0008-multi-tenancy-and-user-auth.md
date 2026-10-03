# ADR-0008: Multi-tenancy and user authentication

- Status: accepted
- Date: 2026-10-03

## Context
Multi-tenant: a tenant (organisation) sees and updates only its own devices. Platform admins create tenants and manage memberships (D8). Login is Google for now, possibly GitHub later (D9).

## Decision
**Authentication:**
- The user API accepts **Google ID tokens** as bearer tokens. They are validated with `coreos/go-oidc`: issuer `https://accounts.google.com`, audience `OAUTH2_CLIENT_ID_GOOGLE`, JWKS signature, expiry, and `email_verified == true`.
- Trusted issuers are a config list (`ONDOTA_OIDC_ISSUERS`). GitHub login is added later by deploying **Dex** in `ondota` and adding its issuer. The domain model doesn't change.
- Clients get tokens with authorization code + PKCE (CLI: loopback redirect). Server-side sessions are not used now.

**Model:**
- `User(id, email, display_name, platform_admin bool)`
- `UserIdentity(issuer, subject → user_id, email, email_verified)`, unique `(issuer, subject)`
- `Tenant(id, slug, name)`
- `Membership(tenant_id, user_id, role)` with role ∈ {`admin`, `operator`, `viewer`}
- `Invitation(tenant_id, email, role)` for users who haven't logged in yet

**First login:**
1. Look up `UserIdentity`.
2. If it's missing and the email is verified, link to an existing `User` with the same email, or create one.
3. Turn matching `Invitation`s into `Membership`s.
4. Emails in `ONDOTA_BOOTSTRAP_ADMINS` get `platform_admin`.

**Permissions:** a fixed role → permission map in code.

| Permission | viewer | operator | admin | platform_admin |
|---|:-:|:-:|:-:|:-:|
| read devices/apps/releases/history | ✓ | ✓ | ✓ | ✓ (all tenants) |
| register/revoke devices, manage applications | | ✓ | ✓ | ✓ |
| manage tenant members | | | ✓ | ✓ |
| create/delete tenants, set platform admins | | | | ✓ |

**Enforcement:**
- Middleware builds a `Principal` (user + memberships) into the context. Inbound service ports check `principal.Can(tenantID, perm)`.
- Every tenant-owned table has `tenant_id NOT NULL`, and every repository method takes `tenantID`.
- A cross-tenant negative test is required for each repository and handler.
- Postgres RLS is not used now; it can be added later as defense in depth.

**Devices:** the device API resolves cert → device → `tenant_id`. Device requests never carry a tenant parameter.

## Consequences
- No IdP to operate for now.
- Google-only login means every user needs a Google account until Dex/GitHub is added.
