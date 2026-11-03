package messaging

//go:generate mockgen -source=publisher.go -destination=../../tests/mocks/nats_mock.go -package=mocks Publisher
import "context"

type Publisher interface {
	Publish(context.Context, string, []byte) error
}
