-- +goose Up
CREATE TABLE article_ai_settings (
 id INTEGER PRIMARY KEY CHECK (id=1),
 classification INTEGER NOT NULL DEFAULT 0 CHECK (classification IN (0,1)),
 featured INTEGER NOT NULL DEFAULT 0 CHECK (featured IN (0,1)),
 prompt TEXT NOT NULL DEFAULT '',
 revision INTEGER NOT NULL DEFAULT 1
);
INSERT INTO article_ai_settings(id) VALUES(1);
CREATE TABLE article_ai_results (
 article_id TEXT NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
 task TEXT NOT NULL CHECK (task IN ('classification','featured')),
 input_key TEXT NOT NULL,
 profile_key TEXT NOT NULL,
 content_hash TEXT NOT NULL,
 model_id TEXT NOT NULL,
 model_revision INTEGER NOT NULL,
 provider_revision INTEGER NOT NULL,
 settings_revision INTEGER NOT NULL,
 importance REAL NOT NULL CHECK (importance BETWEEN 0 AND 1),
 relevance REAL NOT NULL CHECK (relevance BETWEEN 0 AND 1),
 reason TEXT NOT NULL,
 input_tokens INTEGER,
 output_tokens INTEGER,
 evaluated_at INTEGER NOT NULL,
 PRIMARY KEY(article_id,task)
);
CREATE INDEX article_ai_results_featured ON article_ai_results(task,evaluated_at);
CREATE TABLE article_ai_attempts (
 id TEXT PRIMARY KEY,
 started_at INTEGER NOT NULL,
 estimated_tokens INTEGER NOT NULL
);
CREATE INDEX article_ai_attempts_time ON article_ai_attempts(started_at);

CREATE TABLE article_ai_deferrals (
 article_id TEXT NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
 task TEXT NOT NULL,
 until_at INTEGER NOT NULL,
 PRIMARY KEY(article_id,task)
);

-- +goose Down
SELECT 1;
