# AGENTS.md

This file provides guidance to Agent tool when working with code in this repository.

## What is enbu

Keyless `.env` management powered by GitHub. Encrypts secrets with age, stores ciphertext as OCI artifacts on GHCR, and uses GitHub Device Flow for authentication. No shared master key — each team member gets their own age recipient key.

## Commands

```bash
task build          # Build binary → ./enbu
task test           # Unit tests (go test ./...)
task test:scenario  # Scenario tests (build tag: scenario)
task lint           # golangci-lint run ./...
task fmt            # golangci-lint fmt ./...
task check          # lint + test
```

## Architecture

```
main.go              → version injection, signal handling, delegates to pkg/cli
pkg/cli/             → cobra commands: auth, init, add, pull, sync, switch
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

- Secrets are stored per environment as OCI manifests tagged `secrets-{env}` on `ghcr.io/{owner}/{repo}-enbu`
- Recipients are environment-independent: each user's public key is stored as `recipient-{username}-{fingerprint}` (shared across all environments)
- `enbu switch` manages environments (create, switch, delete, rename) with state tracked in `enbu.toml` (shared) and `.enbu.local` (per-user)
- Access control is delegated to OPA/Rego policy evaluated at sync time — not per-environment recipient lists
- `sync` command re-encrypts for all recipients with optimistic concurrency (digest-based conflict detection + exponential backoff retry)
- Private keys are stored via a pluggable keystore backend (OS keyring by default, plaintext file via `ENBU_BACKEND=text`)
- Only age X25519 keys are used — no SSH key support
- No bot/CI decryption — re-encryption requires a human to run `enbu sync`
