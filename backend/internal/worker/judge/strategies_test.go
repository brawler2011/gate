package judge

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/brawler2011/gate/backend/pkg/formats/gfmt"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	pb "github.com/criyle/go-judge/pb"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

type testInputExecutor struct {
	pb.UnimplementedExecutorServer
	t *testing.T
}

func (e *testInputExecutor) Exec(_ context.Context, req *pb.Request) (*pb.Response, error) {
	require.Equal(e.t, []string{"./generator", "42"}, req.GetCmd()[0].GetArgs())
	return pb.Response_builder{Results: []*pb.Response_Result{pb.Response_Result_builder{Status: pb.Response_Result_Accepted, Files: map[string][]byte{"stdout": []byte("generated")}}.Build()}}.Build(), nil
}

func TestCollectFlatTests_ConsumesManifestFilesAndGenerators(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "tests"), 0700))
	for name, data := range map[string]string{"sample.in": "sample", "sample.ans": "sample answer", "edge-case.in": "edge", "edge-case.a": "edge answer", "03.in": "stale generated input"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, "tests", name), []byte(data), 0600))
	}
	prob := &gfmt.Problem{Subtasks: map[string]gfmt.Subtask{
		"z":       {Tests: []gfmt.Test{{Generate: "generator 42"}}},
		"a":       {Dependencies: []string{"samples"}, Tests: []gfmt.Test{{Manual: "edge-case.in"}}},
		"samples": {Tests: []gfmt.Test{{Manual: "sample.in"}}},
	}}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := grpc.NewServer()
	pb.RegisterExecutorServer(server, &testInputExecutor{t: t})
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)
	client, err := sandbox.NewClient(listener.Addr().String())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	strategy := NewStandardStrategy(sandbox.NewSandbox(client, &sandbox.Config{}), nil, &gfmt.GateFormat{Path: dir, Problem: prob}, map[string]sandbox.Executable{"generator_generator": {PrimaryFileID: "generator"}})
	flat := collectFlatTests(prob)
	require.Len(t, flat, 3)
	for i, expected := range []string{"sample", "edge", "generated"} {
		require.Equal(t, i+1, flat[i].TestIndex)
		input, err := strategy.getTestInput(context.Background(), flat[i].Test)
		require.NoError(t, err)
		require.Equal(t, expected, string(input))
		if i < 2 {
			answer, err := strategy.getTestAnswer(context.Background(), flat[i].Test, input)
			require.NoError(t, err)
			require.Equal(t, expected+" answer", string(answer))
		}
	}
}
