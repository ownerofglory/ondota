# ADR-0004: Device identity, enrollment and certificate lifecycle

- Status: accepted
- Date: 2026-10-03

## Context
Devices must authenticate with mTLS only (D6). An existing step-ca (`ca.ownerofglory.com`, in-cluster `step-certificates.step-ca.svc.cluster.local`) has a JWK provisioner `device-provisioner`. Credentials reach the server only through env vars: `STEP_CA_JWK`, `STEP_CA_PASSWORD`, `STEP_CA_PROVISIONER`, `STEP_CA_PEM`, `STEP_CA_PROVIDER_URL` (D10). Certificates should live 180 days (D12).

## Decision
**Registration (user API):** `POST /tenants/{tid}/devices {name}` creates `Device(status=pending)` and returns a one-time password (OTP):
- 20 random bytes, base32, shown once
- only `sha256(otp)` is stored
- TTL 15 min, single use, max 5 failed attempts per device

A new OTP can be issued while the device is still `pending`.

**Enrollment (public, `POST /enroll/v1`):**
1. The agent generates an **ECDSA P-256** key (file mode `0600`, never leaves the device) and a CSR. The subject is free-form; the server ignores it.
2. The agent sends `{otp, csr}`.
3. The server finds the device by OTP hash, checks it is `pending`, unexpired and unused, and verifies the CSR signature (proof of possession) and key type. Then it burns the OTP.
4. The server mints a short-lived (5 min) **JWK one-time token** for `STEP_CA_PROVISIONER`: `sub = <device uuid>`, `sans = ["ondota:device:<uuid>"]`, `aud = <ca>/1.0/sign`, unique `jti`. It calls step-ca `POST /1.0/sign {csr, ott, notAfter: 4320h}`.
5. The device becomes `enrolled` (event `DeviceEnrolled`, storing cert serial + `notAfter`). The response contains the cert chain and the CA bundle the agent pins for the device host.

**Renewal (device API, mTLS):** `POST /device/v1/certificate {csr}` with a **new** key, once 2/3 of the lifetime has passed. The agent retries daily after that. The server checks the device is `enrolled` and signs the same way (event `DeviceCertRenewed`). Renewing through step-ca's `/renew` directly is not used, so ondOTA stays the single point of policy.

**Revocation:** the user API sets `Device(status=revoked)` (event `DeviceRevoked`). The device listener rejects revoked devices on every request (ADR-0003), and renewal is refused. There is no CRL/OCSP: Traefik only checks the chain, and the backend checks the status.

**step-ca configuration:** `device-provisioner` needs `maxTLSCertDuration ≥ 4320h`. The default stays 24h so other users of the provisioner are unaffected. ondOTA always requests `notAfter` explicitly. Until the claim is raised, the server falls back to the provisioner max (configurable `ONDOTA_DEVICE_CERT_TTL`).

**Libraries:** `go.step.sm/crypto` (`jose`, `x509util`) for the OTT and the CSR. step-ca is called over plain HTTP(S) with the root from `STEP_CA_PEM`, instead of the heavy `smallstep/certificates/ca` client.

## Spike result (2026-10-03, `spikes/enroll`)
- A JWK OTT with `aud = <STEP_CA_PROVIDER_URL>/1.0/sign`, `sub = <uuid>` and `sans = ["ondota:device:<uuid>"]` is accepted by step-ca. The external NodePort URL works as the audience.
- A CSR **with empty subject and no SANs** is accepted. The cert gets `CN=<uuid>` and URI SAN `ondota:device:<uuid>` from the token, so the agent doesn't need to know its ID before enrollment.
- Issued by `Step Certificates Intermediate CA`, EKU `serverAuth, clientAuth`.
- `notAfter: 4320h` is rejected with 403 ("authorized maximum … 24h1m0s") until the provisioner claim is raised.

## Consequences
- The device is bound to its tenant in the DB, not in the cert. Moving a device between tenants needs no re-issue.
- A stolen device key works until revocation. The 180-day lifetime is acceptable because revocation is enforced on every request.
- The step-ca provisioner is not in the step-ca Helm values (it was added by editing the ConfigMap), so a `helm upgrade` of step-ca would drop it. This is flagged to the user.
