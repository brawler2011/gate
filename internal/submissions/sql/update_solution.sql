UPDATE solutions
SET state = $1,
    score = $2,
    time_stat = $3,
    memory_stat = $4
WHERE id = $5