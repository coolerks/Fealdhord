-- name: RecordNotificationArticle :exec
INSERT INTO notification_article_events(article_id, discovered_at, published_at, offline_since,resumed_at)
SELECT sqlc.arg(article_id), sqlc.arg(discovered_at), sqlc.narg(published_at),
CASE WHEN f.last_success_at < r.resumed_at THEN r.offline_since ELSE 0 END,
CASE WHEN f.last_success_at < r.resumed_at THEN r.resumed_at ELSE 0 END
FROM notification_runtime r JOIN feeds f ON f.id=sqlc.arg(feed_id) WHERE r.id=1;

-- name: ResumeNotificationRuntime :exec
UPDATE notification_runtime SET offline_since=last_seen,resumed_at=?,last_seen=? WHERE id=1;
-- name: TouchNotificationRuntime :exec
UPDATE notification_runtime SET last_seen=? WHERE id=1;

-- name: CreateNotificationDigestItem :exec
INSERT INTO notification_digest_items(rule_id,rule_revision,article_id,occurrence,recovery,payload)
VALUES(?,?,?,?,?,?) ON CONFLICT(rule_id,article_id) DO NOTHING;
-- name: DueNotificationDigest :many
SELECT * FROM notification_digest_items
WHERE rule_id=? AND rule_revision=? AND queued=0 AND occurrence<=? ORDER BY occurrence,recovery,id LIMIT 100;
-- name: QueueNotificationDigestItem :execrows
UPDATE notification_digest_items SET queued=1 WHERE id=? AND queued=0;

-- name: NotificationCursor :one
SELECT CAST(COALESCE(MAX(seq), 0) AS INTEGER) AS seq FROM notification_article_events;

-- name: ListNotificationEvents :many
SELECT e.seq, e.article_id, e.discovered_at, e.published_at, e.offline_since, e.resumed_at, a.title, a.url, a.summary, substr(a.content_text, 1, 16000) AS body, COALESCE(m.remote_url, CAST('' AS TEXT)) AS cover
FROM notification_article_events e JOIN articles a ON a.id=e.article_id JOIN feeds f ON f.id=a.feed_id LEFT JOIN article_media m ON m.article_id=a.id AND m.status='ready'
WHERE e.seq > sqlc.arg(after_seq)  AND f.archived_at IS NULL
ORDER BY e.seq LIMIT sqlc.arg(page_size);

-- name: ListNotificationArticleCategories :many
SELECT category_id FROM effective_article_categories WHERE article_id=?;

-- name: ListNotificationChannels :many
SELECT * FROM notification_channels WHERE credential_ref<>'' ORDER BY name,id;
-- name: GetNotificationChannel :one
SELECT * FROM notification_channels WHERE id=?;
-- name: CreateNotificationChannel :exec
INSERT INTO notification_channels(id,name,platform,target,credential_ref,enabled,verified,mode) VALUES(?,?,?,?,?,?,?,?);
-- name: UpdateNotificationChannel :execrows
UPDATE notification_channels SET name=?,platform=?,target=?,credential_ref=?,enabled=?,verified=?,revision=revision+1
WHERE id=? AND revision=?;

-- name: ListNotificationRules :many
SELECT * FROM notification_rules WHERE next_at>=0 ORDER BY id;
-- name: GetNotificationRule :one
SELECT * FROM notification_rules WHERE id=?;
-- name: CreateNotificationRule :exec
INSERT INTO notification_rules(id,configuration,enabled,event_cursor,next_at) VALUES(?,?,?,?,?);
-- name: UpdateNotificationRule :execrows
UPDATE notification_rules SET configuration=?,enabled=?,event_cursor=?,next_at=?,revision=revision+1
WHERE id=? AND revision=? AND next_at>=0;
-- name: AdvanceNotificationRule :execrows
UPDATE notification_rules SET event_cursor=?,next_at=? WHERE id=? AND revision=? AND event_cursor=?;

-- name: CreateNotificationDelivery :exec
INSERT INTO notification_deliveries(id,channel_id,source_id,source_revision,payload,due_at,created_at,recipient_id)
VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(id) DO NOTHING;
-- name: ListNotificationDeliveries :many
SELECT d.id,d.channel_id,d.source_id,d.state,d.due_at,d.created_at,d.attempts,d.detail,
CAST(COALESCE(r.target,'') AS TEXT) AS recipient_target
FROM notification_deliveries d LEFT JOIN notification_recipients r ON r.id=d.recipient_id
ORDER BY d.created_at DESC,d.id DESC LIMIT 100;
-- name: NextNotificationDelivery :one
SELECT * FROM notification_deliveries WHERE state='pending' AND due_at<=? ORDER BY due_at,id LIMIT 1;
-- name: BeginNotificationDelivery :execrows
UPDATE notification_deliveries SET state='sending',attempts=attempts+1
WHERE notification_deliveries.id=? AND notification_deliveries.state='pending'
AND EXISTS (SELECT 1 FROM notification_channels c WHERE c.id=notification_deliveries.channel_id AND c.enabled=1 AND c.verified=1)
AND EXISTS (SELECT 1 FROM notification_recipients nr WHERE nr.id=notification_deliveries.recipient_id
  AND nr.channel_id=notification_deliveries.channel_id AND nr.active=1 AND nr.verified=1)
AND CASE
 WHEN notification_deliveries.source_id LIKE 'rule:%' THEN EXISTS (
  SELECT 1 FROM notification_rules r WHERE r.id=substr(notification_deliveries.source_id,6)
    AND r.revision=notification_deliveries.source_revision AND r.enabled=1
 )
 WHEN notification_deliveries.source_id LIKE 'task:%' THEN EXISTS (
  SELECT 1 FROM scheduled_tasks t WHERE t.id=substr(notification_deliveries.source_id,6)
    AND t.revision=notification_deliveries.source_revision AND t.enabled=1
 )
 ELSE 1
END;
-- name: CancelPendingNotificationDelivery :exec
UPDATE notification_deliveries SET state='cancelled',detail='configuration_changed' WHERE id=? AND state='pending';
-- name: FinishNotificationDelivery :exec
UPDATE notification_deliveries SET state=?,due_at=?,detail=? WHERE id=? AND state='sending';
-- name: RecoverNotificationDeliveries :exec
UPDATE notification_deliveries SET state='unknown',detail='interrupted' WHERE state='sending';

-- name: ListScheduledTasks :many
SELECT * FROM scheduled_tasks WHERE next_at>=0 ORDER BY next_at,id;
-- name: GetScheduledTask :one
SELECT * FROM scheduled_tasks WHERE id=? AND next_at>=0;
-- name: CreateScheduledTask :exec
INSERT INTO scheduled_tasks(id,configuration,enabled,next_at) VALUES(?,?,?,?);
-- name: UpdateScheduledTask :execrows
UPDATE scheduled_tasks SET configuration=?,enabled=?,next_at=?,revision=revision+1 WHERE id=? AND revision=?;
-- name: DeleteScheduledTask :execrows
UPDATE scheduled_tasks SET enabled=0,next_at=-1,revision=revision+1 WHERE id=? AND revision=? AND next_at>=0;
-- name: ClaimScheduledTask :execrows
UPDATE scheduled_tasks SET next_at=? WHERE id=? AND revision=? AND next_at=? AND enabled=1;
-- name: StartScheduledRun :exec
INSERT INTO scheduled_runs(task_id,occurrence,task_revision,state) VALUES(?,?,?,'running');
-- name: FinishScheduledRun :execrows
UPDATE scheduled_runs SET state=?,result=?,local_pending=? WHERE task_id=? AND occurrence=? AND state='running';
-- name: ListScheduledRuns :many
SELECT * FROM scheduled_runs WHERE task_id=? ORDER BY occurrence DESC LIMIT 50;
-- name: ListUnfinishedScheduledRuns :many
SELECT * FROM scheduled_runs WHERE state IN ('running','interrupted') ORDER BY occurrence LIMIT 100;
-- name: ResumeScheduledRun :execrows
UPDATE scheduled_runs SET state='running' WHERE task_id=? AND occurrence=? AND state='interrupted';
-- name: RecoverScheduledRuns :exec
UPDATE scheduled_runs SET state='interrupted' WHERE state='running';

-- name: ListTaskArticles :many
SELECT a.id, a.title, a.url, a.summary FROM articles a JOIN feeds f ON f.id=a.feed_id
WHERE a.fetched_at >= sqlc.arg(since) AND a.fetched_at < sqlc.arg(until) AND f.archived_at IS NULL
ORDER BY a.fetched_at DESC, a.id LIMIT 200;

-- name: DeleteNotificationChannel :execrows
UPDATE notification_channels SET enabled=0,verified=0,credential_ref='',revision=revision+1
WHERE id=? AND revision=? AND credential_ref<>'';

-- name: DeleteNotificationRule :execrows
UPDATE notification_rules SET enabled=0,next_at=-1,revision=revision+1
WHERE id=? AND revision=? AND next_at>=0;

-- name: RetryNotificationDelivery :execrows
UPDATE notification_deliveries SET state='pending',due_at=?,detail='',attempts=0
WHERE id=? AND state IN ('unknown','failed');

-- name: ListMatchedNotificationCandidates :many
SELECT DISTINCT e.seq, e.article_id, e.discovered_at, e.published_at, e.offline_since, e.resumed_at, a.title, a.url, a.summary, substr(a.content_text, 1, 16000) AS body, COALESCE(m.remote_url, CAST('' AS TEXT)) AS cover
FROM notification_article_events e JOIN articles a ON a.id=e.article_id JOIN feeds f ON f.id=a.feed_id LEFT JOIN article_media m ON m.article_id=a.id AND m.status='ready'
JOIN notification_rule_candidates c ON c.event_seq=e.seq
JOIN effective_article_categories ec ON ec.article_id=e.article_id
WHERE c.rule_id=sqlc.arg(rule_id) AND c.rule_revision=sqlc.arg(rule_revision) AND c.expires_at>sqlc.arg(now_at)
  AND f.archived_at IS NULL AND instr(CAST(sqlc.arg(category_ids) AS TEXT),json_quote(ec.category_id)) > 0
ORDER BY e.seq LIMIT 100;

-- name: AddNotificationCandidate :exec
INSERT INTO notification_rule_candidates(rule_id,rule_revision,event_seq,expires_at) VALUES(?,?,?,?)
ON CONFLICT(rule_id,event_seq) DO UPDATE SET rule_revision=excluded.rule_revision,expires_at=excluded.expires_at;
-- name: RemoveNotificationCandidate :exec
DELETE FROM notification_rule_candidates WHERE rule_id=? AND event_seq=?;
-- name: PruneNotificationCandidates :exec
DELETE FROM notification_rule_candidates WHERE expires_at<=?;


-- name: ListPendingLocalNotices :many
SELECT r.task_id,r.occurrence,CAST(json_extract(t.configuration,'$.name') AS TEXT) AS title,
substr(r.result, 1, 500) AS body
FROM scheduled_runs r JOIN scheduled_tasks t ON t.id=r.task_id
WHERE r.local_pending=1 AND t.enabled=1 AND t.revision=r.task_revision
ORDER BY r.occurrence LIMIT 30;

-- name: AcknowledgeLocalNotice :exec
UPDATE scheduled_runs SET local_pending=0 WHERE task_id=? AND occurrence=?;

-- name: ListNotificationRecipients :many
SELECT * FROM notification_recipients WHERE channel_id=? AND active=1 ORDER BY target;
-- name: GetNotificationRecipient :one
SELECT * FROM notification_recipients WHERE id=? AND active=1;
-- name: CreateNotificationRecipient :exec
INSERT INTO notification_recipients(id,channel_id,target) VALUES(?,?,?);
-- name: RetireNotificationRecipient :exec
UPDATE notification_recipients SET active=0,verified=0 WHERE id=?;
-- name: VerifyNotificationRecipient :execrows
UPDATE notification_recipients SET verified=1 WHERE notification_recipients.id=? AND active=1
AND EXISTS (SELECT 1 FROM notification_channels c WHERE c.id=notification_recipients.channel_id AND c.revision=? AND c.credential_ref<>'');
-- name: RefreshNotificationChannelVerification :exec
UPDATE notification_channels SET verified=EXISTS(
 SELECT 1 FROM notification_recipients r WHERE r.channel_id=notification_channels.id AND r.active=1 AND r.verified=1
) WHERE notification_channels.id=?;
