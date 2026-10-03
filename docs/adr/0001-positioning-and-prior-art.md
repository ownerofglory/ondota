# ADR-0001: Positioning and borrowed concepts

- Status: accepted
- Date: 2026-10-03

## Context
Mature OTA systems already exist. ondOTA should borrow their proven ideas without taking on their weight.

| System | What it is | What we borrow |
|---|---|---|
| Eclipse hawkBit | Update *server*, device-agnostic. Device-facing **DDI API** (polling) is split from the management API. | API split (device vs. user API); device polls a single resource with a `sleep` hint; *action* = assignment of a version to a device, with feedback states (`proceeding`, `success`, `failure`). |
| Mender | Server + client, full-image A/B and app-level "update modules". | Device inventory reported by the device. Update modules → our `Installer` port per package format. Commit/rollback after a health check. |
| SWUpdate / RAUC | Device-side image installers (A/B slots, bootloader integration). | Not needed now: we update apps, not root filesystems. Our `Installer` port must not exclude a later image installer. |
| balena | Container-based fleet management. | Desired state vs. reported state reconciliation. |
| TUF / Uptane | Metadata framework against compromised repositories; Uptane adds automotive ECU roles. | Keep the desired-state document signable later (M4). Don't trust the artifact host alone; the expected hash comes from the server over mTLS. |
| goreleaser | Release tooling. | Asset naming `<app>_<os>_<arch>.tar.gz` and `checksums.txt` (`<sha256>  <filename>` per line) as the convention for app repos. |

## Decision
- ondOTA is an **app-level** updater with a **desired-state / reported-state** model. The device polls the server for its desired state (hawkBit DDI style) and reports progress.
- **Device API** and **user API** are separate surfaces (see ADR-0003).
- No A/B images, bootloader integration, TUF or Uptane now. The extension points are `ArtifactSource` (server) and `Installer` (agent), plus a signable desired-state document.
- App repos follow the goreleaser conventions.

## Consequences
- Small surface that can be built quickly. Later standards slot in behind the ports instead of requiring a rewrite.
- Not DDI-compatible on the wire. Our resource model mirrors DDI's concepts so an adapter stays possible.
