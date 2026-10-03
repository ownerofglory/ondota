# ADR-0005: Agent install mechanics

- Status: proposed (local spike and a Pi run by the user are still pending)
- Date: 2026-10-03

## Context
Target: Raspberry Pi OS 64-bit, apps run as systemd services (D4). An update must never leave an app half-installed, and a failing release must roll back without human help.

## Decision
- **Layout per app:**
  ```
  <install_dir>/
    releases/<version>/   # extracted release, read-only after install
    current -> releases/<version>
    previous -> releases/<version>   # for rollback
  ```
  The systemd unit's `ExecStart` points at `<install_dir>/current/...`.
- **Flow:**
  1. Download to `<install_dir>/.tmp/` (with resume via `Range`).
  2. Verify size + SHA256 against the server's desired state.
  3. Extract into `releases/<version>.partial`, then rename to `releases/<version>`.
  4. Swap atomically: create `current.new` → `rename(2)` over `current`. Record `previous`.
  5. `systemctl restart <service>`.
  6. Health check: the unit is `active` for `health_window` (default 15s), plus an optional HTTP probe from the manifest.
  7. Success → report `UpdateSucceeded`; prune all but the last N=3 releases.
  8. Failure → swap back to `previous`, restart, report `UpdateRolledBack` with the reason.
- **Crash safety:** the agent writes a small state file (`/var/lib/ondota-agent/state.json`) *before* the swap. At startup, a pending swap without a recorded result triggers the health check again, and rolls back if it fails.
- **Tar safety:** reject absolute paths, `..`, links pointing outside the target, device files, and a total size above a limit. Preserve file modes only (no owners).
- **Ports in the agent:** `Installer` (tar.gz now; deb/image later) and `ServiceManager` (systemd via `systemctl` exec; fakes in tests). D-Bus (`go-systemd`) is not used, to keep it simple.
- **Privileges:** the agent runs as root via its own systemd unit for the MVP. A dedicated user + polkit rule is tracked for M4.
- **Self-update:** the agent is installed with the same layout. A new agent version is swapped in and restarted by systemd. If it doesn't report healthy within the window, a `ExecStartPre` guard rolls `current` back to `previous`. Implemented in M4.

## Consequences
- The extra disk use is N releases per app, which is fine for small Go binaries.
- The whole flow can be tested with temp directories and a fake `ServiceManager`. A real-Pi run is needed only for the systemd integration.
