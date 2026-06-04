# Remove Azure Dependencies

## Context

Herald currently depends on Azure for: AI inference (Azure AI Foundry), document storage (Azure Blob/Azurite), and authentication (Azure Entra/MSAL). The goal is to replace all Azure-specific pieces with portable, Docker-based alternatives while keeping PostgreSQL and adding generic SSO support.

**Decisions:**
- **Auth**: Replace Azure Entra/MSAL with generic OIDC (`oidc-client-ts` frontend, existing `coreos/go-oidc` backend already works for any provider). Works for any SSO provider (Okta, Google Workspace, etc.).
- **Storage**: Replace Azure Blob with **MinIO** (S3-compatible, Docker container, `minio-go/v7` maps 1:1 to existing storage interface).
- **AI**: Switch agent provider from `azure` to `openai` (existing `tauopenai` provider) pointing at any OpenAI-compatible endpoint (Claude, OpenAI, etc.).
- **Database**: Keep PostgreSQL, remove Azure Entra token auth (plain password auth only).
- **Deploy**: Delete all Bicep/ARM infrastructure.

---

## Changes

### 1. Delete entirely
- `deploy/` — all Bicep modules and ARM parameter files
- `compose/azurite.yml` — Azurite local emulator
- `pkg/storage/storage.go` — Azure Blob implementation (replaced by MinIO below)

### 2. `pkg/storage/` — Replace Azure Blob with MinIO
Keep the existing `storage.System` interface and `BlobMeta` types unchanged. Domain code is unaffected.

**`pkg/storage/config.go`** — replace Azure fields with MinIO fields:
```go
type Config struct {
    Endpoint      string // e.g. "localhost:9000"
    AccessKey     string
    SecretKey     string
    UseSSL        bool
    BucketName    string // default: "documents"
    MaxListSize   int32  // default: 50
}
```

**`pkg/storage/storage.go`** — new MinIO implementation using `github.com/minio/minio-go/v7`:
- `minio.New()` → client init
- `client.PutObject()` → `Upload()`
- `client.GetObject()` → `Download()`
- `client.StatObject()` → `Find()` / `Exists()`
- `client.ListObjects()` → `List()` (use `ContinuationToken` for marker pagination)
- `client.RemoveObject()` → `Delete()`
- On `Start()`: create bucket if not exists via `client.MakeBucket()`

**`compose/minio.yml`** — new compose file:
```yaml
services:
  minio:
    image: minio/minio
    container_name: herald-minio
    environment:
      MINIO_ROOT_USER: heraldstore
      MINIO_ROOT_PASSWORD: heraldstorepass
    ports:
      - "9000:9000"
      - "9001:9001"   # console
    volumes:
      - herald-minio:/data
    command: server /data --console-address ":9001"
```

**`docker-compose.yml`** — replace azurite include with minio include.

**`config.docker.json`** — add MinIO defaults:
```json
"storage": {
  "endpoint": "localhost:9000",
  "access_key": "heraldstore",
  "secret_key": "heraldstorepass",
  "use_ssl": false,
  "bucket_name": "documents"
}
```

**`go.mod`** — add `github.com/minio/minio-go/v7`; remove `azure-sdk-for-go/sdk/storage/azblob`.

---

### 3. Auth — disable, leave code in place

**`config.json`** — set auth mode to `"none"` (already the default; confirm it's set).

No changes to `pkg/auth/`, `pkg/middleware/auth.go`, `app/client/core/auth.ts`, or `app/package.json`. All Azure Entra/MSAL code stays commented-in but inactive. SSO will be wired up in a later phase.

**`internal/config/`** — remove env var wiring only for storage (see below); leave all `HERALD_AUTH_*` env vars as-is.

---

### 4. Database — remove token auth

**`pkg/database/config.go`** — remove `TokenLifetime`, `TokenScope`.

**`pkg/database/database.go`** — remove `NewWithCredential` constructor and all `azcore.TokenCredential` / token refresh logic. Plain `pgxpool` with password auth only.

**`go.mod`** — remove `azure-sdk-for-go/sdk/azcore`, `azure-sdk-for-go/sdk/azidentity`, `AzureAD/msal-go`.

---

### 6. Infrastructure and agent config

**`internal/infrastructure/infrastructure.go`**:
- Remove `Credential azcore.TokenCredential` from `Infrastructure` struct
- Remove `tauazure.Register()` from `registerAgentBackends()`
- Remove managed identity branch; always use `database.New()` and `storage.New()`

**`config.json`** — switch agent provider:
```json
"agent": {
  "provider": {
    "name": "openai",
    "base_url": "https://api.anthropic.com/v1",
    "options": {
      "api_key": ""
    }
  },
  "model": { "name": "claude-opus-4-8" }
}
```
Remove `storage` block from `config.json`.

**`go.mod`** — remove `tailored-agentic-units/provider/azure`.

---

## Key Files to Modify

| File | Change |
|------|--------|
| `pkg/storage/config.go` | MinIO fields |
| `pkg/storage/storage.go` | MinIO implementation (replace Azure) |
| `pkg/auth/config.go` | ModeOIDC, remove Azure fields |
| `pkg/middleware/auth.go` | ModeOIDC handling, fix audience/claims |
| `pkg/database/config.go` | Remove TokenLifetime, TokenScope |
| `pkg/database/database.go` | Remove NewWithCredential, token refresh |
| `internal/infrastructure/infrastructure.go` | Remove Credential, tauazure, managed identity |
| `internal/config/config.go` + `agent.go` | Update env var names |
| `cmd/server/modules.go` | OIDC config injection |
| `config.json` | openai provider, remove storage block |
| `config.docker.json` | MinIO connection settings |
| `docker-compose.yml` | Replace azurite with minio |
| `app/package.json` | oidc-client-ts, remove msal-browser |
| `app/client/core/auth.ts` | Rewrite with oidc-client-ts |
| `go.mod` / `go.sum` | Add minio-go, remove azure SDKs |

## Files to Delete

| Path |
|------|
| `deploy/` |
| `compose/azurite.yml` |

## Verification

1. `go mod tidy` — no azure SDK deps remain
2. `mise run build` — clean compile
3. `mise run vet` + `mise run test`
4. `docker compose up -d` — postgres + minio only start
5. MinIO console at `localhost:9001` accessible
6. Set `HERALD_AGENT_BASE_URL` + `HERALD_AGENT_TOKEN` and run classify workflow
7. Set `HERALD_AUTH_MODE=oidc` with a real provider (e.g., Google) and verify login redirect
