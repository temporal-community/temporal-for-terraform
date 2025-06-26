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
	action := flag.String("action", "", "Action to perform (create, destroy, approve, or update)")
	environmentID := flag.String("environment", "", "Environment ID (required for destroy, approve, and update)")
	dnsEntryApproved := flag.Bool("dns-approved", true, "Whether DNS entry is approved (default: true)")
	flag.Parse()

	// Validate action
	if *action != "create" && *action != "destroy" && *action != "approve" && *action != "update" {
		log.Fatal("Action must be either 'create', 'destroy', 'approve', or 'update'")
	}

	// Create Temporal client
	c, err := client.NewClient(client.Options{
		HostPort: client.DefaultHostPort,
	})
	if err != nil {
		log.Fatalf("Unable to create client: %v", err)
	}
	defer c.Close()

	if *action == "approve" || *action == "update" || *action == "destroy" {
		if *environmentID == "" {
			log.Fatal("Environment ID is required for approve, update, or destroy actions")
		}
		workflowID := fmt.Sprintf("terraform-workflow-%s", *environmentID)
		var signalName string
		var signalArg interface{}

		switch *action {
		case "approve":
			signalName = "dns-approval"
			signalArg = true
		case "update":
			signalName = "update"
			signalArg = "update"
		case "destroy":
			signalName = "destroy"
			signalArg = "destroy"
		}

		err = c.SignalWorkflow(context.Background(), workflowID, "", signalName, signalArg)
		if err != nil {
			log.Fatalf("Unable to send %s signal: %v", *action, err)
		}
		log.Printf("Successfully sent %s signal to workflow %s", *action, workflowID)
		return
	}

	// Generate or validate environment ID
	var envID string
	if *action == "create" {
		envID = generateEnvironmentID()
	} else {
		if *environmentID == "" {
			log.Fatal("Environment ID is required for this action")
		}
		envID = *environmentID
	}

	// Start workflow
	workflowOptions := client.StartWorkflowOptions{
		ID:        fmt.Sprintf("terraform-workflow-%s", envID),
		TaskQueue: "terraform-task-queue",
	}

	we, err := c.ExecuteWorkflow(context.Background(), workflowOptions, workflow.InfrastructureWorkflow, envID, *dnsEntryApproved)
	if err != nil {
		log.Fatalf("Unable to start workflow: %v", err)
	}

	log.Printf("Started workflow with ID %s and RunID %s\n", we.GetID(), we.GetRunID())
}
