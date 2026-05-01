# Security Policy

This security policy applies to **rdumanski's fork of Gophish** (`github.com/rdumanski/gophish`).
For vulnerabilities in upstream Gophish (`github.com/gophish/gophish`), please report
to upstream at hi@getgophish.com.

## Supported Versions

Only the latest commit on `main` is supported. Once tagged releases exist, only the most
recent minor version will receive security fixes.

| Version | Status |
|---|---|
| `0.13.0-dev` (main) | Active development |
| `< 0.13.0` (upstream) | Reported to upstream maintainers |

## Reporting a Vulnerability

**Please do NOT open a public GitHub issue for security vulnerabilities.**

Use one of the following private channels:

1. **GitHub private vulnerability reporting** (preferred): open a private advisory at
   <https://github.com/rdumanski/gophish/security/advisories/new>.
2. **Email**: send a description and reproduction steps to the maintainer.

Please include in your report:
- A description of the vulnerability and its impact.
- Steps to reproduce, ideally with a minimal proof of concept.
- Affected version(s) (commit SHA or release tag).
- Your name / handle for the credit line, if you'd like one.

## Coordinated Disclosure

- Initial response: within 7 days of receipt.
- A fix and CVE assignment (where applicable) will be coordinated before any public
  disclosure.
- If the vulnerability also affects upstream Gophish, the report will be forwarded to
  upstream maintainers (with your permission) so the ecosystem benefits from a single fix.

Thank you for helping keep this fork secure.
