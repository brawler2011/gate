package pkg

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const SubmissionsStreamName = "SUBMISSIONS"

// Connection state changes are otherwise silent, including the final close after reconnects run out.
func natsOptions() []nats.Option {
	return []nats.Option{
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			slog.Warn("nats disconnected", "error", err)
		}),
		nats.ReconnectHandler(func(c *nats.Conn) {
			slog.Info("nats reconnected", "url", c.ConnectedUrl())
		}),
		nats.ClosedHandler(func(c *nats.Conn) {
			if err := c.LastError(); err != nil {
				slog.Error("nats connection closed", "error", err)
			}
		}),
		nats.ErrorHandler(func(_ *nats.Conn, sub *nats.Subscription, err error) {
			subject := ""
			if sub != nil {
				subject = sub.Subject
			}
			slog.Error("nats async error", "subject", subject, "error", err)
		}),
	}
}

func NewNatsConn(natsUrl string) (*nats.Conn, error) {
	conn, err := nats.Connect(natsUrl, natsOptions()...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to nats: %w", err)
	}
	return conn, nil
}

func NewNatsJetStream(natsUrl string) (jetstream.JetStream, error) {
	conn, err := NewNatsConn(natsUrl)
	if err != nil {
		return nil, err
	}
	js, err := jetstream.New(conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to create jetstream: %w", err)
	}
	return js, nil
}

// Both publishers and consumers must ensure the stream before use; creation is race-safe.
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

const ContestsStreamName = "CONTESTS"

func EnsureContestsStream(ctx context.Context, js jetstream.JetStream) error {
	_, err := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:     ContestsStreamName,
		Subjects: []string{"contest.>"},
	})
	if err != nil {
		return fmt.Errorf("failed to ensure CONTESTS stream: %w", err)
	}
	return nil
}

func GetSubmissionsLastSequence(ctx context.Context, js jetstream.JetStream) (uint64, error) {
	if js == nil {
		return 0, fmt.Errorf("jetstream is nil")
	}

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
