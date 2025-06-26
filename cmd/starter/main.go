package main

import (
	"context"
	"flag"
	"fmt"
	stdlog "log"
	"math/rand"
	"os"

	"github.com/cdavisafc/terraform-manager/pkg/workflow"
	"go.temporal.io/sdk/client"
)

type temporalLogger struct {
	*stdlog.Logger
}

func (l *temporalLogger) Debug(msg string, keyvals ...interface{}) {
	l.Printf("[DEBUG] %s %v", msg, keyvals)
}

func (l *temporalLogger) Info(msg string, keyvals ...interface{}) {
	l.Printf("[INFO] %s %v", msg, keyvals)
}

func (l *temporalLogger) Warn(msg string, keyvals ...interface{}) {
	l.Printf("[WARN] %s %v", msg, keyvals)
}

func (l *temporalLogger) Error(msg string, keyvals ...interface{}) {
	l.Printf("[ERROR] %s %v", msg, keyvals)
}

func generateEnvironmentID() string {
	return fmt.Sprintf("env-%d", rand.Intn(10000))
}

func main() {
	// Configure logger
	stdLogger := stdlog.New(os.Stdout, "[STARTER] ", stdlog.LstdFlags)
	logger := &temporalLogger{stdLogger}
	logger.Info("Starting workflow...")

	// Parse command line arguments
	action := flag.String("action", "create", "Action to perform: create or destroy")
	environmentID := flag.String("environment", "", "Environment ID (required for destroy, optional for create)")
	flag.Parse()

	// Generate or validate environment ID
	var envID string
	if *action == "create" {
		if *environmentID != "" {
			envID = *environmentID
		} else {
			envID = generateEnvironmentID()
		}
		logger.Info("Using environment ID", "id", envID)
	} else if *action == "destroy" {
		if *environmentID == "" {
			logger.Error("Environment ID is required for destroy action")
			os.Exit(1)
		}
		envID = *environmentID
		logger.Info("Destroying environment", "id", envID)
	} else {
		logger.Error("Unknown action. Use -action=create or -action=destroy")
		os.Exit(1)
	}

	// Create the client object just once per process
	c, err := client.Dial(client.Options{
		HostPort: client.DefaultHostPort,
		Logger:   logger,
	})
	if err != nil {
		logger.Error("Unable to create Temporal client", "error", err)
		os.Exit(1)
	}
	defer c.Close()

	// Create a context for the workflow
	ctx := context.Background()

	// Start the workflow
	workflowOptions := client.StartWorkflowOptions{
		ID:        fmt.Sprintf("terraform-workflow-%s", envID),
		TaskQueue: "terraform-task-queue",
	}

	we, err := c.ExecuteWorkflow(ctx, workflowOptions, workflow.InfrastructureWorkflow, *action, envID)
	if err != nil {
		logger.Error("Unable to start workflow", "error", err)
		os.Exit(1)
	}

	logger.Info("Started workflow", "workflowID", we.GetID(), "runID", we.GetRunID())

	// Wait for the workflow to complete
	var result error
	err = we.Get(ctx, &result)
	if err != nil {
		logger.Error("Unable to get workflow result", "error", err)
		os.Exit(1)
	}

	if result != nil {
		logger.Error("Workflow failed", "error", result)
		os.Exit(1)
	}

	logger.Info("Workflow completed successfully")
}
