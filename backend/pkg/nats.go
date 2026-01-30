package pkg

import (
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

func NewNatsConn(natsUrl string) (*nats.Conn, error) {
	conn, err := nats.Connect(natsUrl)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func NewNatsJetStream(natsUrl string) (jetstream.JetStream, error) {
	conn, err := nats.Connect(natsUrl)
	if err != nil {
		return nil, err
	}
	js, err := jetstream.New(conn)
	if err != nil {
		return nil, err
	}
	return js, nil
}
