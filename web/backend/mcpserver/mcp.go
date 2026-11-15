package mcpserver

import (
	"context"
	"fmt"
	"net/http"

	"go_js_demo/db"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type executeToolInput struct {
	Code string `json:"code" jsonschema:"JavaScript source code to display in the connected editor and execute"`
}

type executeToolOutput struct {
	Output string `json:"output"`
	JobID  string `json:"job_id"`
}

type toolRunner interface {
	Run(ctx context.Context, code string) (runOutcome, error)
}

func newMCPHandler(runner toolRunner) http.Handler {
	server := mcp.NewServer(
		&mcp.Implementation{
			Name:    "go_js",
			Version: "0.1.0",
		},
		&mcp.ServerOptions{
			Instructions: "Use the execute tool to load JavaScript into the connected browser editor and run it. The browser tab must stay open while the tool call is in progress.",
		},
	)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "execute",
		Description: "Display JavaScript in the connected editor and execute it in the go_js runtime. Returns the program output.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input executeToolInput) (*mcp.CallToolResult, executeToolOutput, error) {
		outcome, err := runner.Run(ctx, input.Code)
		if err != nil {
			return nil, executeToolOutput{}, err
		}

		if outcome.Status != db.JobSucceeded {
			message := outcome.ErrorMessage
			if message == "" {
				message = "the interpreter failed to run the code"
			}
			return nil, executeToolOutput{}, fmt.Errorf("execution failed: %s", message)
		}

		return nil, executeToolOutput{
			Output: outcome.Output,
			JobID:  outcome.JobID.String(),
		}, nil
	})

	return mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server {
			return server
		},
		&mcp.StreamableHTTPOptions{
			Stateless:           true,
			JSONResponse:        true,
			MaxRequestBodyBytes: maxMessageSize,
		},
	)
}
