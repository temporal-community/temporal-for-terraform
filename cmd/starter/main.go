package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/cdavisafc/terraform-manager/pkg/workflow"
	"go.temporal.io/sdk/client"
)

type temporalLogger struct {
	*log.Logger
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
	rand.Seed(time.Now().UnixNano())
	return fmt.Sprintf("env-%d", rand.Intn(10000))
}

func main() {
	// Parse command line arguments
	action := flag.String("action", "", "Action to perform (create, destroy, or approve)")
	environmentID := flag.String("environment", "", "Environment ID (required for destroy and approve)")
	dnsEntryApproved := flag.Bool("dns-approved", true, "Whether DNS entry is approved (default: true)")
	flag.Parse()

	// Validate action
	if *action != "create" && *action != "destroy" && *action != "approve" {
		log.Fatal("Action must be either 'create', 'destroy', or 'approve'")
	}

	// Create Temporal client
	c, err := client.Dial(client.Options{
		HostPort: client.DefaultHostPort,
	})
	if err != nil {
		log.Fatalf("Unable to create client: %v", err)
	}
	defer c.Close()

	if *action == "approve" {
		if *environmentID == "" {
			log.Fatal("Environment ID is required for approve action")
		}
		workflowID := fmt.Sprintf("terraform-workflow-%s", *environmentID)
		err = c.SignalWorkflow(context.Background(), workflowID, "", "dns-approval", true)
		if err != nil {
			log.Fatalf("Unable to send approval signal: %v", err)
		}
		log.Printf("Successfully sent DNS approval signal to workflow %s", workflowID)
		return
	}

	// Generate or validate environment ID
	var envID string
	if *action == "create" {
		envID = generateEnvironmentID()
	} else {
		if *environmentID == "" {
			log.Fatal("Environment ID is required for destroy action")
		}
		envID = *environmentID
	}

	// Start workflow
	workflowOptions := client.StartWorkflowOptions{
		ID:        fmt.Sprintf("terraform-workflow-%s", envID),
		TaskQueue: "terraform-task-queue",
	}

	we, err := c.ExecuteWorkflow(context.Background(), workflowOptions, workflow.InfrastructureWorkflow, *action, envID, *dnsEntryApproved)
	if err != nil {
		log.Fatalf("Unable to start workflow: %v", err)
	}

	log.Printf("Started workflow with ID %s and RunID %s\n", we.GetID(), we.GetRunID())

	// Wait for workflow completion
	var result error
	err = we.Get(context.Background(), &result)
	if err != nil {
		log.Fatalf("Workflow failed: %v", err)
	}

	log.Printf("Workflow completed successfully")
}
