package workflow

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// InfrastructureWorkflow orchestrates the deployment of AWS and Cloudflare infrastructure
func InfrastructureWorkflow(ctx workflow.Context, environmentID string, dnsEntryApproved bool) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting infrastructure workflow", "environmentID", environmentID)

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
	var dnsApproved bool

	// Deploy AWS infrastructure
	var awsIP string
	err := workflow.ExecuteActivity(ctx, DeployAWSInfrastructure, "create", environmentID).Get(ctx, &awsIP)
	if err != nil {
		logger.Error("AWS infrastructure deployment failed", "Error", err)
		return err
	}

	// If DNS entry is not approved, wait until it is
	if !dnsEntryApproved {
		logger.Info("Waiting for DNS entry approval with a 1 minute timeout.")
		// Set up a selector to handle DNS approval updates
		selector := workflow.NewSelector(ctx)
		selector.AddReceive(dnsApprovalCh, func(ch workflow.ReceiveChannel, more bool) {
			var approved bool
			ch.Receive(ctx, &approved)
			dnsApproved = approved
			logger.Info("DNS approval status updated", "approved", dnsApproved)
		})

		timerCtx, cancelTimerHandler := workflow.WithCancel(ctx)
		timerFuture := workflow.NewTimer(timerCtx, 30*time.Second)

		timedOut := false
		selector.AddFuture(timerFuture, func(f workflow.Future) {
			timedOut = true
		})

		logger.Info("Waiting for DNS approval signal...")
		for !dnsApproved && !timedOut {
			selector.Select(ctx)
		}
		cancelTimerHandler()

		if timedOut && !dnsApproved {
			logger.Info("DNS approval timed out. Triggering cleanup.")
			err := workflow.ExecuteActivity(ctx, DeployAWSInfrastructure, "destroy", environmentID).Get(ctx, nil)
			if err != nil {
				logger.Error("Failed to destroy AWS infrastructure after timeout.", "Error", err)
			}
			return temporal.NewApplicationError("Approval timed out", "TIMEOUT", "DNS approval was not received within 1 minute.")
		}

		logger.Info("DNS entry approved, proceeding with Cloudflare deployment")
	}

	// Deploy Cloudflare DNS
	err = workflow.ExecuteActivity(ctx, DeployCloudflareDNS, "create", awsIP, environmentID).Get(ctx, nil)
	if err != nil {
		return err
	}
	logger.Info("Initial deployment complete. Workflow is now in a long-running state.")

	// Long-running loop to wait for signals
	destroyCh := workflow.GetSignalChannel(ctx, "destroy")
	updateCh := workflow.GetSignalChannel(ctx, "update")

	var exit bool
	for !exit {
		selector := workflow.NewSelector(ctx)

		selector.AddReceive(destroyCh, func(c workflow.ReceiveChannel, more bool) {
			var signalVal string
			c.Receive(ctx, &signalVal)
			logger.Info("Received destroy signal")
			exit = true
		})

		selector.AddReceive(updateCh, func(c workflow.ReceiveChannel, more bool) {
			var signalVal string
			c.Receive(ctx, &signalVal)
			logger.Info("Received update signal, reapplying Terraform configurations.")

			// Re-apply AWS
			var newAwsIP string
			err := workflow.ExecuteActivity(ctx, DeployAWSInfrastructure, "create", environmentID).Get(ctx, &newAwsIP)
			if err != nil {
				logger.Error("Failed to re-apply AWS infrastructure", "Error", err)
				return // Continue loop
			}
			awsIP = newAwsIP

			// Re-apply Cloudflare
			err = workflow.ExecuteActivity(ctx, DeployCloudflareDNS, "create", awsIP, environmentID).Get(ctx, nil)
			if err != nil {
				logger.Error("Failed to re-apply Cloudflare DNS", "Error", err)
			}
		})

		selector.Select(ctx)
	}

	logger.Info("Destroying infrastructure.")
	err = workflow.ExecuteActivity(ctx, DeployCloudflareDNS, "destroy", awsIP, environmentID).Get(ctx, nil)
	if err != nil {
		logger.Error("Cloudflare DNS destruction failed.", "Error", err)
	}

	err = workflow.ExecuteActivity(ctx, DeployAWSInfrastructure, "destroy", environmentID).Get(ctx, nil)
	if err != nil {
		logger.Error("AWS infrastructure destruction failed.", "Error", err)
	}

	logger.Info("Workflow completed.")
	return nil
}
