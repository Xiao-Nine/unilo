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
