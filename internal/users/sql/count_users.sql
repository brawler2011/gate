SELECT COUNT(*)
FROM users
WHERE (
        $1::text IS NULL
        OR $1 = ''
        OR word_similarity(username, $1) > 0.1
    )
    AND (
        $2::text IS NULL
        OR $2 = ''
        OR role::text = $2
    )