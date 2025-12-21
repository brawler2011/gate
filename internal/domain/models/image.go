package models

import (
	"errors"
	"time"

	"github.com/gate149/core/pkg"
	"github.com/google/uuid"
)

type ImageRecord struct {
	Id        uuid.UUID
	Image     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type CreateImageParams struct {
	Id    uuid.UUID
	Image string
}

func (p CreateImageParams) Validate() error {
	errs := make([]error, 0)

	if !pkg.IsLengthBetween(p.Image, 1, 10485760) {
		errs = append(errs, errors.New("image content must be a valid base64 string"))
	}

	return errors.Join(errs...)
}

type CreateImageInput struct {
	Image string
}
