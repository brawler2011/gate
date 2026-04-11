package pkg

import (
	"context"
	"fmt"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const SubmissionsStreamName = "SUBMISSIONS"

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

// EnsureSubmissionsStream creates the SUBMISSIONS JetStream stream if it does not
// already exist. Both the API server (publisher) and the judge worker (consumer)
// should call this before using the stream so that whichever process starts first
// wins the creation race without error.
func EnsureSubmissionsStream(ctx context.Context, js jetstream.JetStream) error {
	_, err := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:     SubmissionsStreamName,
		Subjects: []string{"submissions.>"},
	})
	if err != nil {
		return fmt.Errorf("failed to ensure SUBMISSIONS stream: %w", err)
	}
	return nil
}

func GetSubmissionsLastSequence(ctx context.Context, js jetstream.JetStream) (uint64, error) {
	stream, err := js.Stream(ctx, SubmissionsStreamName)
	if err != nil {
		return 0, fmt.Errorf("failed to get %s stream: %w", SubmissionsStreamName, err)
	}

	info, err := stream.Info(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get %s stream info: %w", SubmissionsStreamName, err)
	}

	return info.State.LastSeq, nil
}
