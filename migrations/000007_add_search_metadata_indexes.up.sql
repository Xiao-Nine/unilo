CREATE INDEX IF NOT EXISTS idx_search_documents_message_channel_id
    ON search_documents ((metadata->>'channel_id'))
    WHERE source_type = 'message';
