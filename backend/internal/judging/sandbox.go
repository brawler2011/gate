package judging

import (
	"context"

	"github.com/brawler2011/gate/backend/pkg/sandbox"
)

type Sandbox interface {
	Compile(context.Context, []byte, string, map[string][]byte) (sandbox.Executable, error)
	Generate(context.Context, sandbox.Executable, []string) ([]byte, error)
	Test(context.Context, sandbox.Executable, string, []byte, int, int) (*sandbox.RunResult, error)
	Check(context.Context, sandbox.Executable, []byte, []byte, []byte) (*sandbox.CheckerResult, error)
	Interact(context.Context, sandbox.Executable, string, sandbox.Executable, []byte, int, int) (*sandbox.InteractiveResult, error)
}
