BEGIN;

-- 000001_init_auth_channels.up.sql
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username VARCHAR(50) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    nickname VARCHAR(50) NOT NULL,
    avatar_url VARCHAR(500),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE refresh_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(255) NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_refresh_tokens_user_expires ON refresh_tokens(user_id, expires_at DESC);

CREATE TABLE channels (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX idx_channels_deleted_created ON channels(deleted_at, created_at);

CREATE TABLE channel_messages (
    id BIGSERIAL PRIMARY KEY,
    channel_id UUID NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    sender_id UUID NOT NULL REFERENCES users(id),
    reply_to_id BIGINT REFERENCES channel_messages(id),
    msg_type VARCHAR(20) NOT NULL,
    content TEXT NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX idx_channel_messages_channel_id_id ON channel_messages(channel_id, id DESC);
CREATE INDEX idx_channel_messages_sender_created ON channel_messages(sender_id, created_at DESC);
CREATE INDEX idx_channel_messages_reply_to_id ON channel_messages(reply_to_id);

CREATE TABLE channel_reads (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    channel_id UUID NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    last_read_message_id BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, channel_id)
);

CREATE INDEX idx_channel_reads_channel_user ON channel_reads(channel_id, user_id);

-- 000002_create_drops.up.sql
CREATE TABLE drops (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    author_id UUID NOT NULL REFERENCES users(id),
    content TEXT NOT NULL,
    like_count INT NOT NULL DEFAULT 0,
    comment_count INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX idx_drops_created_at ON drops(created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX idx_drops_author_created ON drops(author_id, created_at DESC);

CREATE TABLE drop_likes (
    id BIGSERIAL PRIMARY KEY,
    drop_id UUID NOT NULL REFERENCES drops(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_drop_likes_drop_user UNIQUE (drop_id, user_id)
);

CREATE INDEX idx_drop_likes_user_created ON drop_likes(user_id, created_at DESC);

CREATE TABLE drop_comments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    drop_id UUID NOT NULL REFERENCES drops(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id),
    parent_id UUID REFERENCES drop_comments(id),
    reply_to_user_id UUID REFERENCES users(id),
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX idx_drop_comments_drop_created ON drop_comments(drop_id, created_at ASC) WHERE deleted_at IS NULL;
CREATE INDEX idx_drop_comments_parent_created ON drop_comments(parent_id, created_at ASC) WHERE deleted_at IS NULL;
CREATE INDEX idx_drop_comments_user_created ON drop_comments(user_id, created_at DESC);

-- 000003_create_search_documents.up.sql
CREATE TABLE search_documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_type VARCHAR(30) NOT NULL,
    source_id VARCHAR(100) NOT NULL,
    title TEXT,
    content TEXT NOT NULL,
    tsv TSVECTOR,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_search_documents_source UNIQUE (source_type, source_id)
);

CREATE OR REPLACE FUNCTION search_documents_tsv_update() RETURNS trigger AS $$
BEGIN
    NEW.tsv := to_tsvector('simple', coalesce(NEW.title, '') || ' ' || coalesce(NEW.content, ''));
    NEW.updated_at := now();
    RETURN NEW;
END
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_search_documents_tsv_update
BEFORE INSERT OR UPDATE OF title, content ON search_documents
FOR EACH ROW EXECUTE FUNCTION search_documents_tsv_update();

CREATE INDEX idx_search_documents_tsv ON search_documents USING GIN(tsv);
CREATE INDEX idx_search_documents_type_created ON search_documents(source_type, created_at DESC);

-- 000004_create_workspace_files.up.sql
CREATE TABLE workspace_files (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    parent_id UUID REFERENCES workspace_files(id),
    is_folder BOOLEAN NOT NULL,
    name VARCHAR(255) NOT NULL,
    uploader_id UUID NOT NULL REFERENCES users(id),
    storage_type VARCHAR(20),
    object_key VARCHAR(500),
    size_bytes BIGINT NOT NULL DEFAULT 0,
    mime_type VARCHAR(100),
    file_hash VARCHAR(128),
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    deleted_by UUID REFERENCES users(id)
);

CREATE INDEX idx_workspace_files_parent_folder_name ON workspace_files(parent_id, is_folder, name);
CREATE INDEX idx_workspace_files_file_hash ON workspace_files(file_hash);
CREATE INDEX idx_workspace_files_uploader_created ON workspace_files(uploader_id, created_at DESC);
CREATE INDEX idx_workspace_files_deleted_at ON workspace_files(deleted_at);

CREATE UNIQUE INDEX uq_workspace_files_root_name_active
ON workspace_files(name)
WHERE parent_id IS NULL AND deleted_at IS NULL;

CREATE UNIQUE INDEX uq_workspace_files_parent_name_active
ON workspace_files(parent_id, name)
WHERE parent_id IS NOT NULL AND deleted_at IS NULL;

-- 000005_create_workspace_file_versions.up.sql
CREATE TABLE IF NOT EXISTS workspace_file_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    file_id UUID NOT NULL REFERENCES workspace_files(id) ON DELETE CASCADE,
    editor_id UUID NOT NULL REFERENCES users(id),
    object_key VARCHAR(500) NOT NULL,
    file_hash VARCHAR(128) NOT NULL,
    size_bytes BIGINT NOT NULL,
    mime_type VARCHAR(100),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_workspace_file_versions_file_created
    ON workspace_file_versions(file_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_workspace_file_versions_editor_created
    ON workspace_file_versions(editor_id, created_at DESC);

-- 000006_create_agent_tables.up.sql
CREATE TABLE IF NOT EXISTS agent_conversations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(200) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_agent_conversations_user_updated
    ON agent_conversations(user_id, updated_at DESC);

CREATE TABLE IF NOT EXISTS agent_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL REFERENCES agent_conversations(id) ON DELETE CASCADE,
    role VARCHAR(20) NOT NULL,
    content TEXT NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_agent_messages_role CHECK (role IN ('user', 'assistant', 'system', 'tool'))
);

CREATE INDEX IF NOT EXISTS idx_agent_messages_conversation_created
    ON agent_messages(conversation_id, created_at ASC);

-- 000007_add_search_metadata_indexes.up.sql
CREATE INDEX IF NOT EXISTS idx_search_documents_message_channel_id
    ON search_documents ((metadata->>'channel_id'))
    WHERE source_type = 'message';

-- 000008_create_agent_runs.up.sql
CREATE TABLE IF NOT EXISTS agent_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL REFERENCES agent_conversations(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    user_message_id UUID NOT NULL REFERENCES agent_messages(id) ON DELETE CASCADE,
    assistant_message_id UUID REFERENCES agent_messages(id) ON DELETE SET NULL,
    status VARCHAR(20) NOT NULL,
    error TEXT,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_agent_runs_status CHECK (status IN ('queued', 'running', 'streaming', 'completed', 'failed', 'cancelled'))
);

CREATE INDEX IF NOT EXISTS idx_agent_runs_user_created
    ON agent_runs(user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_agent_runs_conversation_created
    ON agent_runs(conversation_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_agent_runs_status_created
    ON agent_runs(status, created_at ASC);

CREATE INDEX IF NOT EXISTS idx_agent_runs_user_message
    ON agent_runs(user_message_id);

CREATE INDEX IF NOT EXISTS idx_agent_runs_assistant_message
    ON agent_runs(assistant_message_id);

COMMIT;
