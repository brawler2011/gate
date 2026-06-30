-- +goose Up
DROP FUNCTION IF EXISTS get_contest_scoreboard(UUID, BOOLEAN);

ALTER TABLE users DROP COLUMN IF EXISTS name;
ALTER TABLE users DROP COLUMN IF EXISTS surname;
ALTER TABLE users DROP COLUMN IF EXISTS bio;

-- +goose StatementBegin
CREATE FUNCTION get_contest_scoreboard(
    p_contest_id UUID,
    p_realtime BOOLEAN DEFAULT FALSE
) RETURNS TABLE (
    user_id UUID,
    username VARCHAR(70),
    problems_solved INTEGER,
    total_penalty INTEGER,
    last_accepted_at TIMESTAMPTZ,
    problem_results JSONB
)
LANGUAGE plpgsql STABLE AS
$$
DECLARE
    v_freeze_time TIMESTAMPTZ;
    v_contest_start TIMESTAMPTZ;
    v_contest_end TIMESTAMPTZ;
BEGIN
    -- Get contest settings and times
    SELECT 
        start_time,
        end_time,
        CASE 
            WHEN NOT p_realtime 
                AND (settings->>'freeze_monitor')::boolean IS TRUE
                AND end_time IS NOT NULL
            THEN end_time - COALESCE(
                (settings->>'freeze_duration')::interval,
                INTERVAL '1 hour'
            )
            ELSE NULL
        END
    INTO v_contest_start, v_contest_end, v_freeze_time
    FROM contests 
    WHERE id = p_contest_id;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'Contest not found: %', p_contest_id;
    END IF;

    RETURN QUERY
    WITH ranked_submissions AS (
        SELECT 
            s.owner_id,
            s.problem_id,
            s.state,
            s.score,
            s.penalty,
            s.created_at,
            -- Apply freeze: hide results after freeze_time for non-realtime
            CASE 
                WHEN v_freeze_time IS NOT NULL AND s.created_at >= v_freeze_time 
                THEN NULL  -- Frozen state
                ELSE s.state
            END as visible_state,
            CASE 
                WHEN v_freeze_time IS NOT NULL AND s.created_at >= v_freeze_time 
                THEN NULL
                ELSE s.score
            END as visible_score,
            ROW_NUMBER() OVER (
                PARTITION BY s.owner_id, s.problem_id 
                ORDER BY s.created_at ASC
            ) as attempt_num
        FROM submissions s
        WHERE s.contest_id = p_contest_id
          AND s.owner_id IS NOT NULL
    ),
    problem_stats AS (
        SELECT 
            rs.owner_id,
            rs.problem_id,
            COUNT(*) as attempts,
            -- Count frozen attempts (submissions after freeze)
            COUNT(*) FILTER (WHERE rs.visible_state IS NULL) as frozen_attempts,
            -- First accepted submission time
            MIN(CASE WHEN rs.visible_state = 3 THEN rs.created_at END) as first_ac_time,
            -- Calculate penalty: sum of penalties for wrong attempts before AC
            COALESCE(SUM(
                CASE 
                    WHEN rs.visible_state = 3 THEN 0
                    WHEN MIN(CASE WHEN rs.visible_state = 3 THEN rs.created_at END) 
                         OVER (PARTITION BY rs.owner_id, rs.problem_id) IS NOT NULL
                    THEN rs.penalty
                    ELSE 0
                END
            ), 0) as problem_penalty,
            -- Problem is solved if any visible submission is accepted
            BOOL_OR(rs.visible_state = 3) as solved,
            -- Best score among visible submissions
            MAX(rs.visible_score) as best_score
        FROM ranked_submissions rs
        GROUP BY rs.owner_id, rs.problem_id
    ),
    user_stats AS (
        SELECT 
            ps.owner_id,
            COUNT(*) FILTER (WHERE ps.solved) as problems_solved,
            COALESCE(SUM(
                CASE WHEN ps.solved AND v_contest_start IS NOT NULL
                THEN EXTRACT(EPOCH FROM (ps.first_ac_time - v_contest_start))::INTEGER / 60 
                     + ps.problem_penalty
                ELSE 0 END
            ), 0)::INTEGER as total_penalty,
            MAX(ps.first_ac_time) as last_accepted_at,
            jsonb_object_agg(
                ps.problem_id,
                jsonb_build_object(
                    'attempts', ps.attempts,
                    'frozen_attempts', ps.frozen_attempts,
                    'solved', ps.solved,
                    'time_minutes', CASE 
                        WHEN ps.first_ac_time IS NOT NULL AND v_contest_start IS NOT NULL
                        THEN EXTRACT(EPOCH FROM (ps.first_ac_time - v_contest_start))::INTEGER / 60
                        ELSE NULL 
                    END,
                    'penalty', ps.problem_penalty,
                    'score', ps.best_score
                )
            ) as problem_results
        FROM problem_stats ps
        GROUP BY ps.owner_id
    )
    SELECT 
        u.id,
        u.username,
        COALESCE(us.problems_solved, 0)::INTEGER,
        COALESCE(us.total_penalty, 0)::INTEGER,
        us.last_accepted_at,
        COALESCE(us.problem_results, '{}'::jsonb)
    FROM contest_members cm
    INNER JOIN users u ON u.id = cm.user_id
    LEFT JOIN user_stats us ON us.owner_id = u.id
    WHERE cm.contest_id = p_contest_id
      AND cm.role = 'participant'
    ORDER BY 
        COALESCE(us.problems_solved, 0) DESC,
        COALESCE(us.total_penalty, 0) ASC,
        us.last_accepted_at ASC NULLS LAST,
        u.username ASC;
END;
$$;
-- +goose StatementEnd

-- +goose Down
ALTER TABLE users ADD COLUMN name VARCHAR(70) NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN surname VARCHAR(70) NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN bio VARCHAR(500) NOT NULL DEFAULT '';

DROP FUNCTION IF EXISTS get_contest_scoreboard(UUID, BOOLEAN);

-- +goose StatementBegin
CREATE FUNCTION get_contest_scoreboard(
    p_contest_id UUID,
    p_realtime BOOLEAN DEFAULT FALSE
) RETURNS TABLE (
    user_id UUID,
    username VARCHAR(70),
    user_name VARCHAR(100),
    user_surname VARCHAR(100),
    problems_solved INTEGER,
    total_penalty INTEGER,
    last_accepted_at TIMESTAMPTZ,
    problem_results JSONB
)
LANGUAGE plpgsql STABLE AS
$$
DECLARE
    v_freeze_time TIMESTAMPTZ;
    v_contest_start TIMESTAMPTZ;
    v_contest_end TIMESTAMPTZ;
BEGIN
    -- Get contest settings and times
    SELECT 
        start_time,
        end_time,
        CASE 
            WHEN NOT p_realtime 
                AND (settings->>'freeze_monitor')::boolean IS TRUE
                AND end_time IS NOT NULL
            THEN end_time - COALESCE(
                (settings->>'freeze_duration')::interval,
                INTERVAL '1 hour'
            )
            ELSE NULL
        END
    INTO v_contest_start, v_contest_end, v_freeze_time
    FROM contests 
    WHERE id = p_contest_id;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'Contest not found: %', p_contest_id;
    END IF;

    RETURN QUERY
    WITH ranked_submissions AS (
        SELECT 
            s.owner_id,
            s.problem_id,
            s.state,
            s.score,
            s.penalty,
            s.created_at,
            -- Apply freeze: hide results after freeze_time for non-realtime
            CASE 
                WHEN v_freeze_time IS NOT NULL AND s.created_at >= v_freeze_time 
                THEN NULL  -- Frozen state
                ELSE s.state
            END as visible_state,
            CASE 
                WHEN v_freeze_time IS NOT NULL AND s.created_at >= v_freeze_time 
                THEN NULL
                ELSE s.score
            END as visible_score,
            ROW_NUMBER() OVER (
                PARTITION BY s.owner_id, s.problem_id 
                ORDER BY s.created_at ASC
            ) as attempt_num
        FROM submissions s
        WHERE s.contest_id = p_contest_id
          AND s.owner_id IS NOT NULL
    ),
    problem_stats AS (
        SELECT 
            rs.owner_id,
            rs.problem_id,
            COUNT(*) as attempts,
            -- Count frozen attempts (submissions after freeze)
            COUNT(*) FILTER (WHERE rs.visible_state IS NULL) as frozen_attempts,
            -- First accepted submission time
            MIN(CASE WHEN rs.visible_state = 3 THEN rs.created_at END) as first_ac_time,
            -- Calculate penalty: sum of penalties for wrong attempts before AC
            COALESCE(SUM(
                CASE 
                    WHEN rs.visible_state = 3 THEN 0
                    WHEN MIN(CASE WHEN rs.visible_state = 3 THEN rs.created_at END) 
                         OVER (PARTITION BY rs.owner_id, rs.problem_id) IS NOT NULL
                    THEN rs.penalty
                    ELSE 0
                END
            ), 0) as problem_penalty,
            -- Problem is solved if any visible submission is accepted
            BOOL_OR(rs.visible_state = 3) as solved,
            -- Best score among visible submissions
            MAX(rs.visible_score) as best_score
        FROM ranked_submissions rs
        GROUP BY rs.owner_id, rs.problem_id
    ),
    user_stats AS (
        SELECT 
            ps.owner_id,
            COUNT(*) FILTER (WHERE ps.solved) as problems_solved,
            COALESCE(SUM(
                CASE WHEN ps.solved AND v_contest_start IS NOT NULL
                THEN EXTRACT(EPOCH FROM (ps.first_ac_time - v_contest_start))::INTEGER / 60 
                     + ps.problem_penalty
                ELSE 0 END
            ), 0)::INTEGER as total_penalty,
            MAX(ps.first_ac_time) as last_accepted_at,
            jsonb_object_agg(
                ps.problem_id,
                jsonb_build_object(
                    'attempts', ps.attempts,
                    'frozen_attempts', ps.frozen_attempts,
                    'solved', ps.solved,
                    'time_minutes', CASE 
                        WHEN ps.first_ac_time IS NOT NULL AND v_contest_start IS NOT NULL
                        THEN EXTRACT(EPOCH FROM (ps.first_ac_time - v_contest_start))::INTEGER / 60
                        ELSE NULL 
                    END,
                    'penalty', ps.problem_penalty,
                    'score', ps.best_score
                )
            ) as problem_results
        FROM problem_stats ps
        GROUP BY ps.owner_id
    )
    SELECT 
        u.id,
        u.username,
        u.name,
        u.surname,
        COALESCE(us.problems_solved, 0)::INTEGER,
        COALESCE(us.total_penalty, 0)::INTEGER,
        us.last_accepted_at,
        COALESCE(us.problem_results, '{}'::jsonb)
    FROM contest_members cm
    INNER JOIN users u ON u.id = cm.user_id
    LEFT JOIN user_stats us ON us.owner_id = u.id
    WHERE cm.contest_id = p_contest_id
      AND cm.role = 'participant'
    ORDER BY 
        COALESCE(us.problems_solved, 0) DESC,
        COALESCE(us.total_penalty, 0) ASC,
        us.last_accepted_at ASC NULLS LAST,
        u.username ASC;
END;
$$;
-- +goose StatementEnd
