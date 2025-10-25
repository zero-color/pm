package pm

import (
	"context"
	"fmt"
	"testing"

	"cloud.google.com/go/pubsub/v2"
	"cloud.google.com/go/pubsub/v2/pstest"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// TestServer wraps pstest.Server with a pubsub.Client for testing
type TestServer struct {
	srv    *pstest.Server
	conn   *grpc.ClientConn
	Client *pubsub.Client
}

// NewTestServer creates a new test Pub/Sub server and client
func NewTestServer(ctx context.Context, t *testing.T) *TestServer {
	t.Helper()

	// Start the in-memory server
	srv := pstest.NewServer()

	// Create connection to the server
	conn, err := grpc.NewClient(srv.Addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("failed to create grpc client: %v", err)
	}

	// Create pubsub client
	client, err := pubsub.NewClient(ctx, "test-project",
		option.WithGRPCConn(conn),
	)
	if err != nil {
		conn.Close()
		srv.Close()
		t.Fatalf("failed to create pubsub client: %v", err)
	}

	return &TestServer{
		srv:    srv,
		conn:   conn,
		Client: client,
	}
}

// Close cleans up all resources
func (ts *TestServer) Close() error {
	if err := ts.Client.Close(); err != nil {
		return fmt.Errorf("failed to close client: %w", err)
	}
	if err := ts.conn.Close(); err != nil {
		return fmt.Errorf("failed to close connection: %w", err)
	}
	if err := ts.srv.Close(); err != nil {
		return fmt.Errorf("failed to close server: %w", err)
	}
	return nil
}