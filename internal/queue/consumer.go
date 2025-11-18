package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/gate149/core/internal/models"
	"github.com/gate149/core/internal/users"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type Consumer struct {
	redisClient *redis.Client
	usersUC     *users.UsersUseCase
}

type QueueMessage struct {
	Type      string `json:"type"`
	Payload   []byte `json:"payload"`
	CreatedAt string `json:"created_at"`
}

func NewConsumer(redisClient *redis.Client, usersUC *users.UsersUseCase) *Consumer {
	return &Consumer{
		redisClient: redisClient,
		usersUC:     usersUC,
	}
}

func (c *Consumer) StartConsuming(ctx context.Context, queueName string) {
	slog.Info("Starting to consume queue: %s", queueName)

	for {
		select {
		case <-ctx.Done():
			slog.Info("Stopping queue consumer")
			return
		default:
			// Blocking pop from Redis queue
			result, err := c.redisClient.BLPop(ctx, 1*time.Second, queueName).Result()
			if err != nil {
				if errors.Is(err, redis.Nil) {
					// No messages in queue, continue
					continue
				}
				slog.Error("Error reading from queue", "error", err)
				continue
			}

			if len(result) < 2 {
				slog.Error("Invalid queue result", "result", result)
				continue
			}

			messageData := result[1]
			if err := c.processMessage(ctx, messageData); err != nil {
				slog.Error("Error processing message", "error", err)
				if retryErr := c.handleFailedMessage(ctx, queueName, messageData, err); retryErr != nil {
					slog.Error("Failed to handle failed message", "error", retryErr)
				}
			}
		}
	}
}

func (c *Consumer) processMessage(ctx context.Context, messageData string) error {
	var message QueueMessage
	if err := json.Unmarshal([]byte(messageData), &message); err != nil {
		return fmt.Errorf("failed to unmarshal message: %w", err)
	}

	slog.Debug("Processing message", "msg", message)

	switch message.Type {
	case "user_created":
		return c.handleUserCreated(ctx, message)
	default:
		slog.Error("Unknown message type", "type", message.Type)
		return nil
	}
}

func (c *Consumer) handleUserCreated(ctx context.Context, message QueueMessage) error {
	// Parse the Kratos webhook payload
	var kratosPayload struct {
		UserId   string `json:"userId"`
		Username string `json:"username"`
	}

	if err := json.Unmarshal(message.Payload, &kratosPayload); err != nil {
		return fmt.Errorf("failed to parse Kratos payload: %w", err)
	}

	// Create user in tester database
	testerUserId := uuid.New()
	userCreation := models.UserCreation{
		Id:       testerUserId,
		KratosId: &kratosPayload.UserId,
		Username: kratosPayload.Username,
		Role:     "user", // Default role for new users
	}

	_, err := c.usersUC.CreateUser(ctx, &userCreation)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	slog.Info("Successfully created user with Kratos ID", "kratosID", kratosPayload.UserId)
	return nil
}

func (c *Consumer) handleFailedMessage(ctx context.Context, queueName, messageData string, processingErr error) error {
	deadLetterQueue := queueName + ":dlq"

	type FailedMessage struct {
		OriginalMessage string `json:"original_message"`
		Error           string `json:"error"`
		FailedAt        string `json:"failed_at"`
	}

	failed := FailedMessage{
		OriginalMessage: messageData,
		Error:           processingErr.Error(),
		FailedAt:        time.Now().UTC().Format(time.RFC3339),
	}

	failedJSON, err := json.Marshal(failed)
	if err != nil {
		return fmt.Errorf("failed to marshal failed message: %w", err)
	}

	if err := c.redisClient.RPush(ctx, deadLetterQueue, failedJSON).Err(); err != nil {
		return fmt.Errorf("failed to push to dead letter queue: %w", err)
	}

	slog.Warn("Message moved to dead letter queue: %s", deadLetterQueue)
	return nil
}
