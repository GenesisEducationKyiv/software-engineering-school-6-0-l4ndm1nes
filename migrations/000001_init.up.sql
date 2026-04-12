CREATE TABLE IF NOT EXISTS repositories (
    id            SERIAL PRIMARY KEY,
    owner         VARCHAR(255) NOT NULL,
    repo          VARCHAR(255) NOT NULL,
    last_seen_tag VARCHAR(255),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(owner, repo)
);

CREATE TABLE IF NOT EXISTS subscriptions (
    id                SERIAL PRIMARY KEY,
    email             VARCHAR(255) NOT NULL,
    repository_id     INT NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    confirmed         BOOLEAN NOT NULL DEFAULT FALSE,
    confirm_token     VARCHAR(64) NOT NULL UNIQUE,
    unsubscribe_token VARCHAR(64) NOT NULL UNIQUE,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(email, repository_id)
);

CREATE INDEX IF NOT EXISTS idx_subscriptions_email ON subscriptions(email);
CREATE INDEX IF NOT EXISTS idx_subscriptions_confirmed ON subscriptions(confirmed) WHERE confirmed = TRUE;
CREATE INDEX IF NOT EXISTS idx_subscriptions_confirm_token ON subscriptions(confirm_token);
CREATE INDEX IF NOT EXISTS idx_subscriptions_unsubscribe_token ON subscriptions(unsubscribe_token);
