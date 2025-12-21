package handlers

import (
	corev1 "github.com/gate149/contracts/core/v1"
	"github.com/gate149/core/internal/domain/models"
	"github.com/google/uuid"
)

func PaginationDTO(p models.Pagination) corev1.PaginationModel {
	return corev1.PaginationModel{
		Page:  p.Page,
		Total: p.Total,
	}
}

func uuidPtrToUUID(ptr *uuid.UUID) uuid.UUID {
	if ptr == nil {
		return uuid.Nil
	}
	return *ptr
}

func int64PtrToInt64(ptr *int64) int64 {
	if ptr == nil {
		return 0
	}
	return *ptr
}

func int64PtrToIntPtr(ptr *int64) *int {
	if ptr == nil {
		return nil
	}
	val := int(*ptr)
	return &val
}

func int32PtrToInt32(ptr *int32) int32 {
	if ptr == nil {
		return 0
	}
	return *ptr
}
