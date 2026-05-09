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
