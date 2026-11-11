-- name: ReadProblemDraft :one
SELECT to_jsonb(d) || jsonb_build_object(
 'files', COALESCE((SELECT jsonb_object_agg(path,content) FROM problem_draft_files WHERE problem_draft_files.problem_id=d.problem_id),'{}'),
 'components', COALESCE((SELECT jsonb_agg(to_jsonb(c)-'problem_id' ORDER BY id) FROM problem_draft_components c WHERE c.problem_id=d.problem_id),'[]'),
 'subtasks', COALESCE((SELECT jsonb_agg(to_jsonb(s)-'problem_id' || jsonb_build_object('depends_on',
    COALESCE((SELECT jsonb_agg(depends_on ORDER BY depends_on) FROM problem_draft_subtask_dependencies x WHERE x.problem_id=s.problem_id AND x.subtask_id=s.id),'[]')) ORDER BY ordinal)
    FROM problem_draft_subtasks s WHERE s.problem_id=d.problem_id),'[]'),
 'tests', COALESCE((SELECT jsonb_agg(jsonb_build_object('id',t.id,'ordinal',t.ordinal,'subtask_id',t.subtask_id,'sample',t.sample,'points',t.points,
    'input',jsonb_build_object('mode',t.input_mode,'object_id',t.input_object_id,'component_id',t.generator_id,'args',t.generator_args,'seed',t.seed),
    'answer',jsonb_build_object('mode',t.answer_mode,'object_id',t.answer_object_id,'component_id',t.solution_id)) ORDER BY ordinal)
    FROM problem_draft_tests t WHERE t.problem_id=d.problem_id),'[]'),
 'assets', COALESCE((SELECT jsonb_agg(to_jsonb(a)-'problem_id' ORDER BY path) FROM problem_draft_assets a WHERE a.problem_id=d.problem_id),'[]')
)::jsonb AS draft
FROM problem_drafts d WHERE d.problem_id=$1;

-- name: LockProblemDraft :one
SELECT revision FROM problem_drafts WHERE problem_id=$1 FOR UPDATE;

-- name: InsertProblemDraft :exec
INSERT INTO problem_drafts(problem_id,organization_id,title,short_name,type,time_limit_ms,memory_limit_mb)
SELECT id,organization_id,sqlc.arg(title),sqlc.arg(short_name),'pass-fail',1000,256 FROM problems WHERE id=sqlc.arg(problem_id);

-- name: LockStoredObject :one
SELECT * FROM stored_objects WHERE id=$1 FOR UPDATE;

-- name: ReadStoredObject :one
SELECT * FROM stored_objects WHERE id=$1;

-- name: ReadProblemBuild :one
SELECT * FROM problem_builds WHERE id=$1;

-- name: LockProblemBuild :one
SELECT * FROM problem_builds WHERE id=$1 FOR UPDATE;

-- name: NextProblemBuild :one
SELECT id FROM problem_builds WHERE state='queued' ORDER BY created_at,id FOR UPDATE SKIP LOCKED LIMIT 1;
