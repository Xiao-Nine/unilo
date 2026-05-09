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
