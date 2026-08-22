-- name: CreateAuthToken :exec
INSERT INTO auth_tokens (
    id,
    user_id,
    token_type,
    token_hash,
    payload,
    expires_at,
    created_at
) VALUES (
    @id::uuid,
    @user_id::uuid,
    @token_type,
    @token_hash,
    @payload,
    @expires_at::timestamptz,
    NOW()
);

-- name: GetAuthTokenByHash :one
SELECT id, user_id, token_type, token_hash, payload, expires_at, created_at
FROM auth_tokens
WHERE token_hash = @token_hash
  AND token_type = @token_type
LIMIT 1;

-- name: DeleteAuthToken :exec
DELETE FROM auth_tokens
WHERE id = @id::uuid;

-- name: DeleteAuthTokensByUserIdAndType :exec
DELETE FROM auth_tokens
WHERE user_id = @user_id::uuid
  AND token_type = @token_type;

-- name: CleanupExpiredAuthTokens :exec
DELETE FROM auth_tokens
WHERE expires_at < NOW();
