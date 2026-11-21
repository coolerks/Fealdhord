-- +goose Up
ALTER TABLE notification_channels ADD COLUMN mode TEXT NOT NULL DEFAULT '';
CREATE TABLE notification_recipients (
    id TEXT PRIMARY KEY NOT NULL,
    channel_id TEXT NOT NULL REFERENCES notification_channels(id),
    target TEXT NOT NULL,
    active INTEGER NOT NULL DEFAULT 1 CHECK(active IN (0,1)),
    verified INTEGER NOT NULL DEFAULT 0 CHECK(verified IN (0,1))
);
CREATE UNIQUE INDEX notification_active_target ON notification_recipients(channel_id,target) WHERE active=1;
INSERT INTO notification_recipients(id,channel_id,target,verified)
SELECT id,id,target,verified FROM notification_channels WHERE credential_ref<>'';
ALTER TABLE notification_deliveries ADD COLUMN recipient_id TEXT NOT NULL DEFAULT '';
UPDATE notification_deliveries SET recipient_id=channel_id
WHERE EXISTS (SELECT 1 FROM notification_recipients r WHERE r.id=notification_deliveries.channel_id);

-- +goose Down
ALTER TABLE notification_deliveries DROP COLUMN recipient_id;
DROP TABLE notification_recipients;
ALTER TABLE notification_channels DROP COLUMN mode;
