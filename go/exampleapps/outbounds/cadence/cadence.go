package main

import (
	"context"

	"go.uber.org/cadence/client"
)

func foo(c client.Client) {
	c.ExecuteWorkflow(context.Background(), client.StartWorkflowOptions{
		ID:       "workflowID",
		TaskList: "taskList",
	}, "workflowType", "input")
}

func main() {}
