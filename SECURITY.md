# Security Policy

## Supported Versions

This is an actively developed portfolio project. Security fixes are applied to the latest
release and the `main` branch only.

| Version | Supported |
| ------- | --------- |
| latest release / `main` | ✅ |
| older releases | ❌ |

## Reporting a Vulnerability

**Please do not open a public GitHub issue for security vulnerabilities.**

Report privately via GitHub's
[private vulnerability reporting](https://github.com/VidIsWandering/secure-payment-gateway/security/advisories/new)
(Security tab → *Report a vulnerability*), or by email to **tomnguyen023@gmail.com**.

Please include:

- A description of the issue and its impact
- Steps to reproduce (request samples, headers, payloads)
- The affected commit or release
- Any suggested fix, if you have one

You can expect an acknowledgement within **72 hours** and a status update within
**7 days**. Once a fix is released, you will be credited in the release notes unless you
prefer to remain anonymous.

## Scope

In scope:

- Authentication and request signing (HMAC, JWT, API keys)
- Replay protection (nonce / timestamp checks)
- Wallet balance integrity (race conditions, double spending, idempotency bypass)
- Encryption of secrets and balances at rest
- SSRF via merchant webhook URLs
- Injection, XSS or access-control issues in the API or dashboard

Out of scope:

- Findings that rely on the development defaults in `docker-compose.yml`
  (e.g. Grafana `admin/admin`, sample `SPG_AES_KEY`) — these are for local use only
- Denial of service through volumetric traffic
- Vulnerabilities in third-party dependencies without a demonstrated impact on this project
  (these are tracked automatically by Dependabot, `govulncheck` and CodeQL)
