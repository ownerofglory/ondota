# Changelog

All notable changes to this project are documented here.

## [Unreleased]

### Added
- Investigation plan for the ondOTA server and agent (`docs/plan/investigation.md`).
- Plan v2: recorded user decisions; device enrollment via OTP + CSR signed by step-ca; device API over mTLS only.
- Plan v3: multi-tenancy with OIDC authentication and role-based access; step-ca configuration via environment variables.
- Plan v4: Google/GitHub login via Dex, 180-day device certificates, roles confirmed.
- Plan v5: Google OIDC login (Dex deferred); 180-day max for device certificates on step-ca.
- ADRs 0001–0008.
- Spikes: device enrollment via step-ca JWK one-time token; mTLS on `devices.ondota.ownerofglory.com` via Traefik.
