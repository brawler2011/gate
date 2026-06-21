package sandbox

import (
	"context"
	"fmt"
	"time"

	pb "github.com/criyle/go-judge/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client wraps the gRPC connection and proto client for go-judge.
type Client struct {
	conn       *grpc.ClientConn
	grpcClient pb.ExecutorClient
}

// NewClient establishes a new gRPC connection to go-judge at the given address.
func NewClient(addr string) (*Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(100*1024*1024)), // 100MB
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to go-judge gRPC server: %w", err)
	}

	return &Client{
		conn:       conn,
		grpcClient: pb.NewExecutorClient(conn),
	}, nil
}

// Close terminates the gRPC connection.
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// Exec sends an execution request to go-judge.
func (c *Client) Exec(ctx context.Context, req *pb.Request) (*pb.Response, error) {
	if c.grpcClient == nil {
		return nil, fmt.Errorf("gRPC client is not initialized")
	}
	return c.grpcClient.Exec(ctx, req)
}

// Helper constructors for go-judge Request_File fields.

func newMemoryFile(content []byte) *pb.Request_File {
	return pb.Request_File_builder{
		Memory: pb.Request_MemoryFile_builder{
			Content: content,
		}.Build(),
	}.Build()
}

func newCachedFile(fileID string) *pb.Request_File {
	return pb.Request_File_builder{
		Cached: pb.Request_CachedFile_builder{
			FileID: fileID,
		}.Build(),
	}.Build()
}

func newPipeCollector(name string, maxLimit int64) *pb.Request_File {
	return pb.Request_File_builder{
		Pipe: pb.Request_PipeCollector_builder{
			Name: name,
			Max:  maxLimit,
		}.Build(),
	}.Build()
}
