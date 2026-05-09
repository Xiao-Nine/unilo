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
