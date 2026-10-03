# ADR-0003: API surfaces and mTLS termination

- Status: accepted
- Date: 2026-10-03

## Context
Users authenticate with OIDC (ADR-0008). Devices authenticate with client certificates (ADR-0004), but have none until enrollment. The cluster runs Traefik and cert-manager (`letsencrypt-production` ClusterIssuer). A working PoC exists in namespace `mtls-poc`: a `TLSOption` with `RequireAndVerifyClientCert` and a `passTLSClientCert` middleware.

Traefik selects TLS options per router, by SNI. It cannot require a client cert on one path and make it optional on another under the same host.

## Decision
| Host | TLS | Surfaces |
|---|---|---|
| `ondota.ownerofglory.com` | Let's Encrypt, no client cert | `/api/v1` (user API, OIDC bearer), `/enroll/v1` (OTP + CSR), `/hooks/github` |
| `devices.ondota.ownerofglory.com` | Let's Encrypt server cert, **`RequireAndVerifyClientCert`** against the step-ca root + intermediate | `/device/v1` |

- **Traefik terminates mTLS** (as in `mtls-poc`). The `passTLSClientCert` middleware (`pem: true`) forwards the leaf in `X-Forwarded-Tls-Client-Cert`.
- The server runs **two HTTP listeners** (`:8080` public, `:8081` device). Only the device router targets `:8081`, and only the device listener reads the forwarded-cert header. The public listener never trusts it. A NetworkPolicy restricts ingress to Traefik.
- The device listener parses the PEM again, checks it chains to the configured CA bundle and is within validity (defense in depth), extracts the device ID from the URI SAN `ondota:device:<uuid>`, and loads the device. Requests from devices that are revoked or unknown are rejected (`403`).
- Contracts are in `api/openapi-user.yaml` and `api/openapi-device.yaml`.

## Alternatives
- TLS passthrough (`IngressRouteTCP`) with Go terminating mTLS: no trusted header, but the server has to manage its own certs and Traefik can't route by path. Revisit if the header trust model ever becomes a concern.
- A single host with `VerifyClientCertIfGiven`: rejected, because the device API would rely on application code alone.

## Consequences
- Two Ingress/IngressRoute objects, one `TLSOption`, one `Middleware`, and a Secret with the device CA bundle, all in `ondota`.
- Header spoofing is prevented structurally (separate listener plus NetworkPolicy), not by convention.
