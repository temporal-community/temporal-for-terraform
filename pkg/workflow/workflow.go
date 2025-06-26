package workflow

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// InfrastructureWorkflow orchestrates the deployment of AWS and Cloudflare infrastructure
func InfrastructureWorkflow(ctx workflow.Context, action string, environmentID string, dnsEntryApproved bool) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting infrastructure workflow", "action", action, "environmentID", environmentID, "dnsEntryApproved", dnsEntryApproved)

	// Set up activity options
	options := workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute * 2,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute * 10,
			MaximumAttempts:    10,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, options)

	// Create a channel to receive DNS approval updates
	dnsApprovalCh := workflow.GetSignalChannel(ctx, "dns-approval")

	// Set up a selector to handle DNS approval updates
	selector := workflow.NewSelector(ctx)
	selector.AddReceive(dnsApprovalCh, func(ch workflow.ReceiveChannel, more bool) {
		var approved bool
		ch.Receive(ctx, &approved)
		dnsEntryApproved = approved
		logger.Info("DNS approval status updated", "approved", dnsEntryApproved)
	})

	// Deploy AWS infrastructure
	var awsIP string
	err := workflow.ExecuteActivity(ctx, DeployAWSInfrastructure, action, environmentID).Get(ctx, &awsIP)
	if err != nil {
		return err
	}

	// If DNS entry is not preapproved, wait until approval comes in
	if !dnsEntryApproved {
		logger.Info("Waiting for DNS entry approval")
		for !dnsEntryApproved {
			selector.Select(ctx)
		}
		logger.Info("DNS entry approved, proceeding with Cloudflare deployment")
	}

	// Ensure we have the proper activity context before executing Cloudflare activity
	ctx = workflow.WithActivityOptions(ctx, options)

	// Deploy Cloudflare DNS
	err = workflow.ExecuteActivity(ctx, DeployCloudflareDNS, action, awsIP, environmentID).Get(ctx, nil)
	if err != nil {
		return err
	}

	return nil
}
