# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository status

This repository contains product/API/database/infrastructure specifications for `unilo`, a Vue 3 + Vite frontend skeleton under `web/`, and a Go backend under `cmd/`, `internal/`, and `pkg/`.

The intended direction is a collaborative all-in-one MVP with a Vue 3 + Vite frontend, Go + Gin backend, GORM, PostgreSQL, Redis Pub/Sub, and MinIO/S3-compatible object storage. The current backend slice implements service verification/auth, channel/message REST APIs, and a basic WebSocket hub.

Key source-of-truth documents:
- [requirement.md](requirement.md) — product requirements for channels, async feed, workspace, and AI assistant.
- [api.md](api.md) — REST and WebSocket API contract.
- [schema.md](schema.md) — database table design.
- [docker-compose-infra.yaml](docker-compose-infra.yaml) — local infrastructure services.
- [prototype.png](prototype.png) — UI prototype reference.

## Commands

Infrastructure:

```bash
# Start local PostgreSQL, Redis, and MinIO
docker compose -f docker-compose-infra.yaml up -d

# Stop local infrastructure
docker compose -f docker-compose-infra.yaml down

# Validate the compose file
docker compose -f docker-compose-infra.yaml config

# Follow service logs
docker compose -f docker-compose-infra.yaml logs -f postgres redis minio
```

Frontend web app:

```bash
# Install frontend dependencies
npm install --prefix web

# If the user-level npm cache has permission issues, use a project-local cache
npm install --prefix web --cache ./.npm-cache

# Start Vite dev server
npm run dev --prefix web

# Build frontend
npm run build --prefix web

# Preview production build
npm run preview --prefix web
```

Backend:

The backend uses the current local machine Go toolchain version; `go.mod` is currently set to Go 1.25.1.

```bash
# Download/update Go dependencies
go mod tidy

# Run backend tests
go test ./...

# Start the backend server
go run ./cmd/server

# Apply SQL migrations with golang-migrate CLI
migrate -path migrations -database "$DATABASE_URL" up

# Roll back one migration
migrate -path migrations -database "$DATABASE_URL" down 1
```

## High-level architecture

`unilo` is a shared collaboration app with four primary product areas plus an AI assistant:

- **Channels**: Discord/Kook-like real-time chat channels. All users can create, rename, and delete channels; channel lists should sync to online users. Messages support text, emoji, images, code blocks, replies, and WebSocket delivery.
- **Drops**: Memos-like asynchronous feed. Posts support Markdown, likes, and nested comments.
- **Workspace**: Shared virtual file system. Users share a global directory tree with upload/download, rename, move, soft-delete recycle bin, preview, and Milkdown-backed editable Markdown files.
- **Service onboarding/Auth**: On first launch, the frontend/app asks for backend service URL and secret key, verifies them through `/auth/verify`, then shows login/register. Registration is open after service verification; user sessions use Access Token + Refresh Token.
- **AI assistant**: Invoked from channels or a dedicated AI interface. MVP behavior should use an OpenAI-compatible backend proxy, PostgreSQL full-text search, current channel/site context, and return answers; action execution can be expanded later.

## API conventions

The REST API contract in [api.md](api.md) uses `${server_url}/api/v1` as the base path after the frontend/app has verified the user-provided backend service URL and secret key. All REST responses should use the documented envelope:

```json
{
  "code": 200,
  "msg": "success",
  "data": {}
}
```

Authentication conventions from the API spec:
- First-launch service onboarding calls `${server_url}/api/v1/auth/verify` with the user-entered secret key.
- Verification success lets the frontend/app store the service URL and enter login/register; verification failure keeps the user on onboarding.
- Public auth endpoints are `/auth/verify`, `/auth/register`, `/auth/login`, `/auth/refresh`, and `/auth/logout`.
- All other user endpoints require `Authorization: Bearer <Access_Token>`; `/auth/me` is protected even though it uses the `/auth` prefix.
- WebSocket connections use `${server_url}/api/v1/ws?token=<Access_Token>`.
- Login returns Access Token + Refresh Token; refresh token persistence is modeled in `schema.md`.

Major API modules:
- `/auth/verify`, `/auth/register`, `/auth/login`, `/auth/refresh`, `/auth/logout`, `/auth/me`
- `/channels`, `/channels/:channel_id/messages`
- `/drops`, `/drops/:drop_id/like`, `/drops/:drop_id/comments`
- `/workspace/files`, `/workspace/folders`, `/workspace/files/check`, `/workspace/files/upload`
- `/agent/conversations`, `/search`

## Data model

The schema in [schema.md](schema.md) defines these core tables:

- `users` and `refresh_tokens`: login identity, nickname, avatar, password hash, and refresh-token lifecycle.
- `channels` and `channel_messages`: chat channel metadata and ordered messages. `channel_messages.id` is `BIGSERIAL` and is used for cursor pagination.
- `drops`, `drop_likes`, and `drop_comments`: Markdown posts, denormalized like counts, and nested comments.
- `workspace_files` and optional `workspace_file_versions`: self-referential file/folder tree with object storage metadata and editable text-file version history.
- `agent_conversations`, `agent_messages`, and `search_documents`: AI conversation history and searchable site content.

Local infrastructure in [docker-compose-infra.yaml](docker-compose-infra.yaml):
- PostgreSQL 15 listens on container port `5432`; default host binding is `127.0.0.1:8001` unless `POSTGRES_PORT` overrides it.
- Redis 7 listens on container port `6379`; default host binding is `127.0.0.1:8002` unless `REDIS_PORT` overrides it.
- MinIO listens on container ports `9000` and `9001`; default host bindings are `127.0.0.1:8003` and `127.0.0.1:8004` unless MinIO port env vars override them.
