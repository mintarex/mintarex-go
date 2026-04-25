# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability in this SDK, please report it
privately so we can assess and patch it before public disclosure.

**Email:** security@mintarex.com

Please include:
- A description of the vulnerability and its potential impact
- Steps to reproduce (PoC code, request samples, etc.)
- The SDK version and runtime environment
- Your name / handle for credit (optional)

We aim to:
- **Acknowledge** receipt within **48 hours**
- **Triage** and provide an initial assessment within **5 business days**
- **Patch** confirmed high/critical issues within **30 days**, with a
  coordinated disclosure timeline shared with you

Do **not** open a public GitHub issue for security reports.

## Supported Versions

Only the **latest minor version** receives security updates. Older versions
are deprecated; please upgrade to the current release.

## Scope

In scope:
- Vulnerabilities in this SDK's source code
- Insecure defaults that put callers at risk
- Cryptographic mistakes (HMAC signing, webhook verification, etc.)
- Dependency vulnerabilities that affect SDK users

Out of scope:
- Vulnerabilities in the Mintarex API itself (report directly to
  security@mintarex.com — we'll route appropriately)
- Vulnerabilities in third-party packages we depend on (report upstream
  first, then to us)
- Issues that require a compromised local environment to exploit

## Safe Harbor

We will not pursue legal action against researchers who:
- Make a good-faith effort to comply with this policy
- Do not access, modify, or destroy data beyond what is necessary to
  demonstrate the vulnerability
- Give us reasonable time to respond before any public disclosure

Thank you for helping keep Mintarex and our users safe.
