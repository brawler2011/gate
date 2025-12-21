package interfaces

type NatsPublisher interface {
	Publish(subject string, data []byte) error
}
