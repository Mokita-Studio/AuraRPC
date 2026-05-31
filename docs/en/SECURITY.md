# SECURITY.md

# Security and Privacy

AuraRPC operates under explicit and verifiable privacy guarantees within the code. This document defines the threat model and how to report vulnerabilities.

## Privacy Guarantees

1. **Zero telemetry.** The binary does not send usage data, crash reports, or analytics.
2. **Minimal outbound network.** The only network egress is an optional update check: a single unauthenticated HTTPS GET to the GitHub releases API, which sends nothing about the user and can be disabled (`check_updates` in `config.json`). It never downloads or installs anything. Everything else is local IPC with Discord (named pipe or Unix socket).
3. **No authentication.** AuraRPC does not request tokens, does not use OAuth, and does not access the account. It only sends payloads to the local Discord client.
4. **No middlemen.** External servers or cloud databases are not used.
5. **Local data only.** Everything is saved in the per-user config directory (`%APPDATA%\AuraRPC\` on Windows). The user has full control over these files (see [CONFIGURATION.md](CONFIGURATION.md)).

## Threat Model

**What is mitigated:**
- **Unstable client:** Unexpected Discord closures are handled via an automatic reconnection loop with backoff.
- **Duplicate execution:** Blocked at the kernel level by a single-instance lock (Win32 Mutex).

**What is out of scope:**
- Attackers with local access (root/administrator) to the machine.
- A previously compromised local Discord client.
- Manipulated executables downloaded from unofficial sources.

## Code Integrity

- Dependencies are verified using cryptographic hashes in `go.mod` and `go.sum`.
- CI runs strict checks (`go vet` and `go test -race`) on every push.
- Build tools (`tc-hib/winres`) are not injected into the final execution binary.

## Reporting Vulnerabilities

**Do not** open public issues for security problems. Send a private report to:

`contact@mokitastudio.com`

Include:
- Problem description.
- Steps to reproduce or proof of concept.
- Estimated impact.

Receipt will be confirmed within 72 hours and patch releases will be coordinated safely.