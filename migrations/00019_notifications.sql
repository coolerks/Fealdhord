-- +goose Up
CREATE TABLE notification_article_events (
    seq INTEGER PRIMARY KEY AUTOINCREMENT,
    article_id TEXT NOT NULL UNIQUE REFERENCES articles(id) ON DELETE CASCADE,
    discovered_at INTEGER NOT NULL,
    published_at INTEGER,
    offline_since INTEGER NOT NULL DEFAULT 0,
    resumed_at INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX notification_event_time ON notification_article_events(discovered_at, seq);
CREATE TABLE notification_runtime (
    id INTEGER PRIMARY KEY CHECK(id=1),
    last_seen INTEGER NOT NULL,
    offline_since INTEGER NOT NULL,
    resumed_at INTEGER NOT NULL
);
INSERT INTO notification_runtime VALUES(1,0,0,0);

CREATE TABLE notification_channels (
    id TEXT PRIMARY KEY NOT NULL,
    name TEXT NOT NULL,
    platform TEXT NOT NULL,
    target TEXT NOT NULL,
    credential_ref TEXT NOT NULL,
    enabled INTEGER NOT NULL CHECK (enabled IN (0,1)),
    verified INTEGER NOT NULL CHECK (verified IN (0,1)),
    revision INTEGER NOT NULL DEFAULT 1
);
CREATE TABLE notification_rules (
    id TEXT PRIMARY KEY NOT NULL,
    configuration TEXT NOT NULL CHECK(json_valid(configuration)),
    enabled INTEGER NOT NULL CHECK(enabled IN (0,1)),
    revision INTEGER NOT NULL DEFAULT 1,
    event_cursor INTEGER NOT NULL,
    next_at INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE notification_rule_candidates (
    rule_id TEXT NOT NULL REFERENCES notification_rules(id),
    rule_revision INTEGER NOT NULL,
    event_seq INTEGER NOT NULL REFERENCES notification_article_events(seq) ON DELETE CASCADE,
    expires_at INTEGER NOT NULL,
    PRIMARY KEY(rule_id, event_seq)
);
CREATE INDEX notification_candidate_expiry ON notification_rule_candidates(expires_at);
CREATE TABLE notification_digest_items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    rule_id TEXT NOT NULL REFERENCES notification_rules(id),
    rule_revision INTEGER NOT NULL,
    article_id TEXT NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
    occurrence INTEGER NOT NULL,
    recovery INTEGER NOT NULL,
    payload TEXT NOT NULL CHECK(json_valid(payload)),
    queued INTEGER NOT NULL DEFAULT 0 CHECK(queued IN (0,1)),
    UNIQUE(rule_id, article_id)
);
CREATE INDEX notification_digest_due ON notification_digest_items(rule_id, rule_revision, queued, occurrence, recovery, id);
CREATE TABLE notification_deliveries (
    id TEXT PRIMARY KEY NOT NULL,
    channel_id TEXT NOT NULL REFERENCES notification_channels(id),
    source_id TEXT NOT NULL,
    source_revision INTEGER NOT NULL,
    payload TEXT NOT NULL CHECK(json_valid(payload)),
    state TEXT NOT NULL DEFAULT 'pending' CHECK(state IN ('pending','sending','accepted','failed','unknown','cancelled')),
    due_at INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    detail TEXT NOT NULL DEFAULT ''
);
CREATE INDEX notification_delivery_due ON notification_deliveries(state, due_at);
CREATE TABLE scheduled_tasks (
    id TEXT PRIMARY KEY NOT NULL,
    configuration TEXT NOT NULL CHECK(json_valid(configuration)),
    enabled INTEGER NOT NULL CHECK(enabled IN (0,1)),
    revision INTEGER NOT NULL DEFAULT 1,
    next_at INTEGER NOT NULL
);
CREATE TABLE scheduled_runs (
    task_id TEXT NOT NULL REFERENCES scheduled_tasks(id),
    occurrence INTEGER NOT NULL,
    task_revision INTEGER NOT NULL,
    state TEXT NOT NULL,
    result TEXT NOT NULL DEFAULT '',
    local_pending INTEGER NOT NULL DEFAULT 0 CHECK(local_pending IN (0,1)),
    PRIMARY KEY(task_id, occurrence)
);
-- +goose Down
DROP TABLE scheduled_runs;
DROP TABLE scheduled_tasks;
DROP TABLE notification_deliveries;
DROP TABLE notification_digest_items;
DROP TABLE notification_rule_candidates;
DROP TABLE notification_rules;
DROP TABLE notification_channels;
DROP TABLE notification_article_events;
DROP TABLE notification_runtime;
