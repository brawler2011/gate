package interfaces

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -source=nats.go -destination=../../../tests/mocks/nats_mock.go -package=mocks Publisher

import "context"

type Publisher interface {
	Publish(ctx context.Context, subject string, data []byte) error
}

