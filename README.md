# 💃 enbu

A `.env` management tool that works entirely within GitHub.

## Why

Development requires sensitive information like API keys and database passwords, but existing approaches have problems:

- Slack/Discord/Email lack E2EE
    - Confusing characters like `1`, `I`, `l` and italic rendering cause copy-paste errors
    - Every change requires notifying everyone manually
    - Even if you encrypt: the delivery channel for the password or decryption key is often insecure
- Dedicated secret managers?
    - External services come with cost and operational overhead
        - AWS/Google Cloud/1Password require contracts and account management
        - Significant organizational burden in both cost and operations
- Just commit it to Git!
    - Ciphertext persists permanently in Git history
    - Future algorithm weaknesses could allow retroactive decryption

## Features

- **GitHub-only** — No dependency on external platforms
- **E2E encrypted** — Only each member's local private key can decrypt
- **Simple CLI** — After setup, just `enbu add` and `enbu pull`
<!-- Planned -->
<!--- **Secret leak prevention** — Prevent committing .env files or hardcoded secrets -->
<!--- **Tamper detection** — Sigstore-based signing and verification to detect tampering -->
<!--- **Policy control** — OPA/Rego-based policy enforcement -->

## Install

```bash
go install github.com/yashikota/enbu@latest
```

Or download a binary from [Releases](https://github.com/yashikota/enbu/releases).

## Quick Start

### 1. Authenticate

```bash
enbu auth login
```

Log in to GitHub.

### 2. Initialize the repository

```bash
cd your-repo
enbu init
```

Run once per user per repository. This automatically:

- Generates an X25519 key pair
- Stores the private key in the OS keychain
- Registers the public key on GHCR
- Creates `enbu.toml`
- Updates `.gitignore`

### 3. Add or edit secrets

```bash
enbu add DATABASE_URL "postgres://..."
enbu add API_KEY "sk-..."
enbu edit API_KEY "sk-new..."

# Environment-specific secrets
enbu add --env dev DATABASE_URL "postgres://dev/..."
enbu add --env prod DATABASE_URL "postgres://prod/..."
```

`add` creates a new secret and fails if the key already exists. Use `edit` to update an existing secret.

### 4. Delete secrets

```bash
enbu delete API_KEY
```

### 5. Pull secrets

```bash
enbu pull  # Writes to .env file
enbu pull --env dev  # Writes to the configured output for dev
```

### 6. Add a team member

A new member runs `enbu init` inside the repository to enter join mode and register their public key.  
An existing member then runs `enbu sync` locally to re-encrypt secrets for the new recipient.

## Environments

Define environments and their output files in `enbu.toml`:

```toml
version = "0.1"

[env.default]
output = ".env"

[env.dev]
output = ".env.dev"

[env.prod]
output = ".env.prod"
```

Use `--env` with `init`, `add`, `edit`, `delete`, `pull`, and `sync`. Each environment has its own encrypted secret bundle and recipient list. Without `--env`, enbu uses `default`.

## Key Storage

Private keys are stored in the OS secure storage:

| OS | Backend |
|----|---------|
| macOS | Keychain |
| Linux | Secret Service (GNOME Keyring / KWallet) |
| Windows | Credential Manager |

For environments without a keychain (containers, headless servers), specify a fallback via environment variable:

```bash
export ENBU_BACKEND=text  # Plaintext file (0600 permissions)
```

## How It Works

```
GHCR (ghcr.io/{owner}/{repo}-enbu)
├── recipient-{user}-{fingerprint}      ← Public keys for default
├── recipient-dev-{user}-{fingerprint}  ← Public keys for dev
├── secrets-default                     ← Encrypted default secrets
└── secrets-dev                         ← Encrypted dev secrets
```

1. `enbu add` — Creates a new secret, encrypts for all recipients' public keys, and pushes as an OCI image artifact
2. `enbu edit` — Updates an existing secret in the encrypted bundle and pushes the updated artifact
3. `enbu delete` — Removes a secret from the encrypted bundle and pushes the updated artifact
4. `enbu pull` — Pulls ciphertext, decrypts with your private key, writes to `.env`
5. `enbu sync` — Re-encrypts with the current recipient list when members are added or removed

### Authentication & Initialization Flow

```mermaid
sequenceDiagram
    participant User
    participant CLI as enbu CLI
    participant GitHub as GitHub OAuth
    participant GHCR

    User->>CLI: enbu auth login
    CLI->>GitHub: Request Device Code
    GitHub-->>CLI: User Code + Verification URI
    CLI-->>User: Display code & open browser
    User->>GitHub: Authorize in browser
    CLI->>GitHub: Poll for token
    GitHub-->>CLI: Access Token
    CLI-->>User: ✓ Authenticated

    User->>CLI: enbu init
    CLI->>CLI: Generate age X25519 key pair
    CLI->>CLI: Store private key in OS keychain
    CLI->>GHCR: Register public key as recipient-{user}-{fingerprint}
    GHCR-->>CLI: Done
    CLI-->>User: ✓ Initialized
```

### Secret Addition Flow

```mermaid
sequenceDiagram
    participant User
    participant CLI as enbu CLI
    participant GHCR

    User->>CLI: enbu add KEY VALUE
    CLI->>GHCR: Fetch all recipient public keys
    GHCR-->>CLI: Public key list
    CLI->>CLI: Encrypt with age for all public keys
    CLI->>GHCR: Push to secrets-default
    GHCR-->>CLI: Done
    CLI-->>User: ✓ Secret added
```

### Member Addition & Sync Flow

```mermaid
sequenceDiagram
    participant New as New Member
    participant Member as Existing Member
    participant CLI as enbu CLI
    participant GHCR

    New->>CLI: enbu init (join mode)
    CLI->>CLI: Generate age key pair
    CLI->>GHCR: Register public key as recipient-{user}
    CLI-->>New: ✓ Key registered

    Member->>CLI: enbu sync
    CLI->>GHCR: Fetch all recipient public keys
    GHCR-->>CLI: Public key list
    CLI->>GHCR: Pull secrets-default
    GHCR-->>CLI: Ciphertext
    CLI->>CLI: Decrypt with private key → re-encrypt for all public keys
    CLI->>GHCR: Update secrets-default

    New->>CLI: enbu pull
    CLI->>GHCR: Pull secrets-default
    GHCR-->>CLI: Ciphertext
    CLI->>CLI: Decrypt with private key
    CLI-->>New: Write .env
```
