package pg

import (
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func Offset(page int32, pageSize int32) int32 {
	return (page - 1) * pageSize
}

func ordinalToLetter(ordinal *int32) *string {
	if ordinal == nil {
		return nil
	}
	ord := int(*ordinal)
	letter := ""
	for ord >= 0 {
		letter = string(rune('A'+(ord%26))) + letter
		ord = ord/26 - 1
	}
	return &letter
}

func uuidPtrToPgtypeUUID(id *uuid.UUID) pgtype.UUID {
	if id == nil {
		return pgtype.UUID{Valid: false}
	}
	return pgtype.UUID{Bytes: *id, Valid: true}
}

func pgtypeUUIDToPtr(id pgtype.UUID) *uuid.UUID {
	if !id.Valid {
		return nil
	}
	val := uuid.UUID(id.Bytes)
	return &val
}

func pgtypeTimestamptzToPtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	val := t.Time
	return &val
}
