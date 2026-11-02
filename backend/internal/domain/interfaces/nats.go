package interfaces

//go:generate mockgen -source=nats.go -destination=../../../tests/mocks/nats_mock.go -package=mocks Publisher

import "context"

type Publisher interface {
	Publish(ctx context.Context, subject string, data []byte) error
}

