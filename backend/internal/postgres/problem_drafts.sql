-- name: ReadProblemDraft :one
SELECT problem_id,revision,title,short_name,type,grading,time_limit_ms,memory_limit_mb,max_score,content FROM problem_drafts WHERE problem_id=$1;

-- name: LockProblemDraft :one
SELECT revision FROM problem_drafts WHERE problem_id=$1 FOR UPDATE;

-- name: DraftObjectIDs :one
SELECT object_ids FROM problem_drafts WHERE problem_id=$1;

-- name: InsertProblemDraft :exec
INSERT INTO problem_drafts(problem_id,organization_id,title,short_name,type,grading,time_limit_ms,memory_limit_mb)
SELECT id,organization_id,sqlc.arg(title),sqlc.arg(short_name),'standard','pass-fail',1000,256 FROM problems WHERE id=sqlc.arg(problem_id);

-- name: ReadStoredObject :one
SELECT * FROM stored_objects WHERE id=$1;

-- name: ReadProblemBuild :one
SELECT * FROM problem_builds WHERE id=$1;

-- name: LockProblemBuild :one
SELECT * FROM problem_builds WHERE id=$1 FOR UPDATE;

-- name: NextProblemBuild :one
SELECT id FROM problem_builds WHERE state='queued' ORDER BY created_at,id FOR UPDATE SKIP LOCKED LIMIT 1;
