package workflow

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// InfrastructureWorkflow orchestrates the deployment of AWS and Cloudflare infrastructure
func InfrastructureWorkflow(ctx workflow.Context, action string, environmentID string) error {
	options := workflow.ActivityOptions{
		StartToCloseTimeout: time.Hour,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute * 10,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, options)

	// Deploy AWS infrastructure
	var awsIP string
	err := workflow.ExecuteActivity(ctx, DeployAWSInfrastructure, action, environmentID).Get(ctx, &awsIP)
	if err != nil {
		return err
	}

	// Deploy Cloudflare DNS
	err = workflow.ExecuteActivity(ctx, DeployCloudflareDNS, action, awsIP, environmentID).Get(ctx, nil)
	if err != nil {
		return err
	}

	return nil
}
