# AGENTS.md

This file provides guidance to Agent tool when working with code in this repository.

## What is enbu

Keyless `.env` management powered by GitHub. Encrypts secrets with age, stores ciphertext as OCI artifacts on GHCR, and uses GitHub Device Flow for authentication. No shared master key — each team member gets their own age recipient key.

## Notes

- After writing code, always write tests for the relevant areas.
- Force-pushes are prohibited.
- After changing code, always run `task build`, `task test:all`, and `task check`.
- When a Linear task is provided, use a branch name like `feat/enbu-01`.

## Commands

```bash
task build          # Build binary
task test           # Unit tests
task test:scenario  # Scenario tests
task check          # lint + test
```

## Architecture

```
main.go              → version injection, signal handling, delegates to pkg/cli
pkg/cli/             → cobra commands: auth, init, add, pull, sync
pkg/config/          → repo detection (git remote), enbu.toml, XDG data dir
pkg/auth/            → GitHub Device Flow OAuth, token persistence
pkg/age/             → key generation, encrypt/decrypt with age (X25519 only)
pkg/keystore/        → pluggable private key storage (OS keyring or plaintext file)
pkg/bundle/          → JSON marshal/unmarshal of secret map, .env serialization
pkg/oci/             → push/pull OCI artifacts to GHCR (oras-go), tag listing, digest checks
pkg/provider/github/ → GitHub API client (org detection)
test/                → scenario tests (build tag: scenario)
```

## Key design decisions

- Secrets are stored as a single OCI manifest tagged `secrets-default` on `ghcr.io/{owner}/{repo}-enbu`
- Each recipient's public key is stored as a separate tag `recipient-{username}-{fingerprint}`
- `sync` command re-encrypts for all recipients with optimistic concurrency (digest-based conflict detection + exponential backoff retry)
- Private keys are stored via a pluggable keystore backend (OS keyring by default, plaintext file via `ENBU_BACKEND=text`)
- Only age X25519 keys are used — no SSH key support
- No bot/CI decryption — re-encryption requires a human to run `enbu sync`
