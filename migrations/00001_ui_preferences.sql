-- +goose Up
CREATE TABLE app_settings (
    key TEXT PRIMARY KEY,
    value_json TEXT NOT NULL CHECK(json_valid(value_json)),
    schema_version INTEGER NOT NULL CHECK(schema_version > 0),
    revision INTEGER NOT NULL DEFAULT 0 CHECK(revision >= 0),
    updated_at INTEGER NOT NULL
);
INSERT INTO app_settings (key, value_json, schema_version, revision, updated_at)
VALUES ('ui_preferences', '{}', 1, 0, 0);

-- +goose Down
DROP TABLE app_settings;
