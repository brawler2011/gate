INSERT INTO contests (id, title, created_by)
VALUES ($1, $2, $3)
RETURNING id