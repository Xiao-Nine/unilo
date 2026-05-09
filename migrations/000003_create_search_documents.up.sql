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
