-- name: CreateFeed :exec
INSERT INTO feeds(id,title,url,normalized_url,created_at,updated_at) VALUES(?,?,?,?,?,?);
-- name: GetFeedByURL :one
SELECT * FROM feeds WHERE normalized_url=?;
-- name: RestoreFeed :exec
UPDATE feeds SET archived_at=NULL,updated_at=?,revision=revision+1 WHERE id=?;
-- name: ArchiveFeed :execrows
UPDATE feeds SET archived_at=?,updated_at=?,revision=revision+1 WHERE id=? AND revision=?;
-- name: ListFeeds :many
SELECT * FROM feeds WHERE archived_at IS NULL ORDER BY sort_position,title,id;
-- name: FindIdentity :one
SELECT article_id FROM article_identities WHERE feed_id=? AND kind=? AND value=?;
-- name: CreateArticle :exec
INSERT INTO articles(id,feed_id,guid,title,url,normalized_url,author,summary,content_html,content_text,cjk_terms,published_at,updated_at,fetched_at,sort_at,content_hash,dedup_key)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?);
-- name: UpdateArticle :exec
UPDATE articles SET guid=COALESCE(?,guid),title=?,url=?,normalized_url=COALESCE(?,normalized_url),author=?,summary=?,content_html=?,content_text=?,cjk_terms=?,published_at=COALESCE(?,published_at),updated_at=?,content_hash=?,revision=revision+1 WHERE id=?;
-- name: InsertIdentity :exec
INSERT INTO article_identities(article_id,feed_id,kind,value) VALUES(?,?,?,?) ON CONFLICT(feed_id,kind,value) DO NOTHING;
-- name: GetArticle :one
SELECT * FROM articles WHERE id=?;
-- name: DeleteArticle :execrows
DELETE FROM articles WHERE id=?;
-- name: ListArticles :many
SELECT a.id,a.title,a.url,a.author,substr(a.summary,1,500) AS summary,a.sort_at,a.feed_id,f.title AS feed_title,
 COALESCE(r.is_read,0) AS is_read,CASE WHEN fav.article_id IS NULL THEN 0 ELSE 1 END AS is_favorite,
 m.url AS cover_url,m.width AS cover_width,m.height AS cover_height,m.alt AS cover_alt
FROM articles a JOIN feeds f ON f.id=a.feed_id
LEFT JOIN reading_states r ON r.article_id=a.id LEFT JOIN favorites fav ON fav.article_id=a.id
LEFT JOIN article_media m ON m.article_id=a.id AND m.status='ready'
ORDER BY a.sort_at DESC,a.id DESC LIMIT ?;
-- name: SetRead :exec
INSERT INTO reading_states(article_id,is_read,read_at,updated_at) VALUES(?,?,?,?)
ON CONFLICT(article_id) DO UPDATE SET is_read=excluded.is_read,read_at=excluded.read_at,updated_at=excluded.updated_at,revision=reading_states.revision+1;
-- name: AddFavorite :exec
INSERT INTO favorites(article_id,created_at) VALUES(?,?) ON CONFLICT(article_id) DO NOTHING;
-- name: RemoveFavorite :exec
DELETE FROM favorites WHERE article_id=?;
-- name: CreateCategory :exec
INSERT INTO categories(id,parent_id,name,normalized_name,created_at,updated_at) VALUES(?,?,?,?,?,?);
-- name: MoveCategory :execrows
UPDATE categories SET parent_id=?,updated_at=?,revision=revision+1 WHERE id=? AND revision=?;
-- name: SetMembership :exec
INSERT INTO article_categories(article_id,category_id,source,state,confidence,created_at,updated_at) VALUES(?,?,?,?,?,?,?)
ON CONFLICT(article_id,category_id,source) DO UPDATE SET state=excluded.state,confidence=excluded.confidence,updated_at=excluded.updated_at;
-- name: ResetManualMembership :exec
DELETE FROM article_categories WHERE article_id=? AND category_id=? AND source='manual';
-- name: EffectiveMembers :many
SELECT article_id FROM effective_article_categories WHERE category_id=? ORDER BY article_id;
-- name: InheritFeedCategories :exec
INSERT INTO article_categories(article_id,category_id,source,state,created_at,updated_at)
SELECT ?,category_id,'feed','included',?,? FROM feed_categories WHERE feed_id=?
ON CONFLICT(article_id,category_id,source) DO UPDATE SET state='included',updated_at=excluded.updated_at;
-- name: GetCategory :one
SELECT * FROM categories WHERE id=?;
-- name: MoveChildrenToParent :exec
UPDATE categories SET parent_id=?,updated_at=?,revision=revision+1 WHERE parent_id=?;
-- name: DeleteCategory :execrows
DELETE FROM categories WHERE id=?;
-- name: SetFeedCategory :exec
INSERT INTO feed_categories(feed_id,category_id,created_at) VALUES(?,?,?) ON CONFLICT(feed_id,category_id) DO NOTHING;
-- name: GetFeed :one
SELECT * FROM feeds WHERE id=?;
-- name: RemoveFeedCategories :exec
DELETE FROM feed_categories WHERE feed_id=?;
-- name: RemoveInheritedMemberships :exec
DELETE FROM article_categories WHERE source='feed' AND article_id IN (SELECT id FROM articles WHERE feed_id=?);
-- name: ApplyFeedCategories :exec
INSERT INTO article_categories(article_id,category_id,source,state,created_at,updated_at)
SELECT a.id,fc.category_id,'feed','included',?,? FROM articles a JOIN feed_categories fc ON fc.feed_id=a.feed_id WHERE a.feed_id=?
ON CONFLICT(article_id,category_id,source) DO UPDATE SET state='included',updated_at=excluded.updated_at;
-- name: ListAllFeeds :many
SELECT * FROM feeds ORDER BY sort_position,title,id;
-- name: ListDueFeeds :many
SELECT * FROM feeds WHERE archived_at IS NULL AND (next_check_at IS NULL OR next_check_at<=?) ORDER BY next_check_at,id LIMIT 500;
-- name: SaveFeedDetails :execrows
UPDATE feeds SET title=?,url=?,normalized_url=?,site_url=?,description=?,updated_at=?,revision=revision+1,
 etag=CASE WHEN normalized_url<>? THEN '' ELSE etag END,
 last_modified=CASE WHEN normalized_url<>? THEN '' ELSE last_modified END,
 next_check_at=CASE WHEN normalized_url<>? THEN NULL ELSE next_check_at END
WHERE id=? AND revision=?;
-- name: SaveFeedSuccess :exec
UPDATE feeds SET etag=?,last_modified=?,last_checked_at=?,last_success_at=?,next_check_at=?,error_count=0,last_error='',last_error_code='',updated_at=? WHERE id=?;
-- name: SaveFeedNotModified :exec
UPDATE feeds SET last_checked_at=?,last_success_at=?,next_check_at=?,error_count=0,last_error='',last_error_code='',updated_at=? WHERE id=?;
-- name: SaveFeedFailure :exec
UPDATE feeds SET last_checked_at=?,next_check_at=?,error_count=error_count+1,last_error=?,last_error_code=?,updated_at=? WHERE id=? AND archived_at IS NULL AND normalized_url=?;
-- name: ListCategories :many
SELECT * FROM categories ORDER BY sort_position,name,id;
-- name: ListFeedCategories :many
SELECT category_id FROM feed_categories WHERE feed_id=? ORDER BY category_id;
