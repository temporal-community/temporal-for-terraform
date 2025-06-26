package main

import (
	stdlog "log"
	"os"

	"github.com/cdavisafc/terraform-manager/pkg/workflow"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
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

func main() {
	// Configure logger
	stdLogger := stdlog.New(os.Stdout, "[WORKER] ", stdlog.LstdFlags)
	logger := &temporalLogger{stdLogger}
	logger.Info("Starting worker...")

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

	// This worker hosts both Workflow and Activity functions
	w := worker.New(c, "terraform-task-queue", worker.Options{})

	// Register workflow and activities
	w.RegisterWorkflow(workflow.InfrastructureWorkflow)
	w.RegisterActivity(workflow.DeployAWSInfrastructure)
	w.RegisterActivity(workflow.DeployCloudflareDNS)

	logger.Info("Worker started, listening on terraform-task-queue")

	// Start listening to the Task Queue
	err = w.Run(worker.InterruptCh())
	if err != nil {
		logger.Error("Unable to start worker", "error", err)
		os.Exit(1)
	}
}
