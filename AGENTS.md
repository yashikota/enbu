# AGENTS.md

This file provides guidance to Agent tool when working with code in this repository.

## What is enbu

Keyless `.env` management powered by GitHub. Encrypts secrets with age, stores ciphertext as OCI artifacts on GHCR, and uses GitHub Device Flow for authentication. No shared master key — each team member gets their own age recipient key.

## Commands

```bash
task build          # Build binary → ./enbu
task test           # Unit tests (go test ./...)
task test:integration  # Integration tests (build tag: integration)
task test:e2e       # E2E tests (build tag: e2e)
task lint           # golangci-lint run ./...
task fmt            # golangci-lint fmt ./...
task check          # lint + test
```

## Architecture

```
main.go              → version injection, signal handling, delegates to internal/cli
internal/cli/        → cobra commands: auth, init, add, pull, sync
internal/config/     → repo detection (git remote), enbu.toml, XDG data dir
internal/auth/       → GitHub Device Flow OAuth, token persistence
internal/age/        → key generation, encrypt/decrypt with age (X25519 only)
internal/keystore/   → pluggable private key storage (OS keyring or plaintext file)
internal/bundle/     → JSON marshal/unmarshal of secret map, .env serialization
internal/oci/        → push/pull OCI artifacts to GHCR (oras-go), tag listing, digest checks
internal/github/     → GitHub API client (org detection)
```

## Key design decisions

- Secrets are stored as a single OCI manifest tagged `secrets-default` on `ghcr.io/{owner}/{repo}-enbu`
- Each recipient's public key is stored as a separate tag `recipient-{username}-{fingerprint}`
- `sync` command re-encrypts for all recipients with optimistic concurrency (digest-based conflict detection + exponential backoff retry)
- Private keys are stored via a pluggable keystore backend (OS keyring by default, plaintext file via `ENBU_BACKEND=text`)
- Only age X25519 keys are used — no SSH key support
- No bot/CI decryption — re-encryption requires a human to run `enbu sync`
