# ADR-0002: Release ingestion and artifact sources

- Status: accepted
- Date: 2026-10-03

## Context
Releases are GitHub Releases of public repos for now (D1). Later: MinIO/S3 (private, presigned URLs), Debian packages, custom formats. Devices only talk to the ondOTA server (D2), and the version policy is "latest stable" (D5).

## Decision
- **Ingestion is server-side, through two paths into one idempotent use case `IngestRelease(repo, tag)`:**
  1. GitHub `release` webhook (actions `published`, `released`, `edited`, `deleted`), verified with `X-Hub-Signature-256` (HMAC secret per repo registration).
  2. A periodic reconcile poll (`GET /repos/{o}/{r}/releases`) with `If-None-Match`/ETag, which catches missed webhooks. An optional `GITHUB_TOKEN` raises the rate limit (60/h → 5000/h); conditional 304s don't count against it.
- Draft and pre-release releases are stored but are not "stable". The desired state uses the newest stable release by semver (tag `vX.Y.Z`).
- For each release, the server reads the matching asset and parses `checksums.txt`. A release without a checksum for the configured asset is marked `invalid` and never deployed.
- **Ports:**
  - `ArtifactSource` (outbound): `ListReleases`, `GetRelease`, `ResolveDownload(artifact) → {url, sha256, size, expiresAt}`.
    - GitHub adapter: returns the public `browser_download_url`.
    - MinIO/S3 adapter (later): returns a presigned URL.
  - The device always receives `{url, sha256, size}`, so moving to private storage needs no agent change.
- Ingestion appends a `ReleasePublished` event. A subscriber recomputes the desired state of devices whose manifest includes that application.

## Consequences
- The server needs a public webhook endpoint (`ondota.ownerofglory.com/hooks/github`).
- Assets are downloaded straight from GitHub's CDN, not proxied, which keeps server traffic low. The trusted hash comes from the server, not from the download host.
