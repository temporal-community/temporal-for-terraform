package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/cdavisafc/terraform-manager/pkg/terraform"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
)

// DeployAWSInfrastructure handles the AWS infrastructure deployment
func DeployAWSInfrastructure(ctx context.Context, action string, environmentID string) (string, error) {
	shouldFail := false
	if shouldFail {
		return "", temporal.NewApplicationError("forced failure for retry testing", "TEST_FAILURE")
	}
	logger := activity.GetLogger(ctx)
	logger.Info("Starting AWS infrastructure deployment", "action", action, "environmentID", environmentID)

	// Get the current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("error getting current directory: %v", err)
	}

	// Set up environment-specific state directory
	awsDir := filepath.Join(cwd, "terraform-configs", "aws-terraform")
	stateDir := filepath.Join(awsDir, "state", environmentID)

	// Create state directory if it doesn't exist
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create state directory: %v", err)
	}

	// Initialize Terraform with state file in the environment-specific directory
	cmd := exec.Command("terraform", "init", "-reconfigure", "-backend-config", fmt.Sprintf("path=%s/terraform.tfstate", stateDir))
	cmd.Dir = awsDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("terraform init failed: %v", err)
	}

	// Apply or destroy based on action
	if action == "create" {
		// If the state file exists, untaint the resource before applying
		// This needs more thorough testing
		stateFile := filepath.Join(stateDir, "terraform.tfstate")
		if _, err := os.Stat(stateFile); err == nil {
			logger.Info("State file exists, untainting resource")
			if err := terraform.Untaint(awsDir); err != nil {
				return "", fmt.Errorf("terraform untaint failed: %v", err)
			}
		}
		if err := terraform.Apply(awsDir, true); err != nil {
			return "", fmt.Errorf("terraform apply failed: %v", err)
		}
	} else if action == "destroy" {
		if err := terraform.Destroy(awsDir, true); err != nil {
			return "", fmt.Errorf("terraform destroy failed: %v", err)
		}
	}

	// Get the public IP if we're creating
	if action == "create" {
		cmd := exec.Command("terraform", "output", "-json")
		cmd.Dir = awsDir
		output, err := cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("failed to get terraform output: %v", err)
		}

		logger.Info("Terraform output retrieved", "output", string(output))

		var result struct {
			WebServerPublicIP struct {
				Value string `json:"value"`
			} `json:"web_server_public_ip"`
		}

		if err := json.Unmarshal(output, &result); err != nil {
			return "", fmt.Errorf("failed to parse terraform output: %v", err)
		}

		logger.Info("AWS public IP retrieved", "ip", result.WebServerPublicIP.Value)
		return result.WebServerPublicIP.Value, nil
	}

	return "", nil
}

// DeployCloudflareDNS handles the Cloudflare DNS configuration
func DeployCloudflareDNS(ctx context.Context, action string, awsIP string, environmentID string) error {
	shouldFail := false
	if shouldFail {
		return fmt.Errorf("forced failure for retry testing")
	}
	logger := activity.GetLogger(ctx)
	logger.Info("Starting Cloudflare DNS deployment", "action", action, "ip", awsIP, "environmentID", environmentID)

	// Get the current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("error getting current directory: %v", err)
	}

	// Set up environment-specific state directory
	cfDir := filepath.Join(cwd, "terraform-configs", "cf-terraform")
	stateDir := filepath.Join(cfDir, "state", environmentID)

	// Create state directory if it doesn't exist
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %v", err)
	}

	// Initialize Terraform with state file in the environment-specific directory
	cmd := exec.Command("terraform", "init", "-reconfigure", "-backend-config", fmt.Sprintf("path=%s/terraform.tfstate", stateDir))
	cmd.Dir = cfDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("terraform init failed: %v", err)
	}

	// Get Cloudflare credentials from environment
	apiToken := os.Getenv("CLOUDFLARE_API_TOKEN")
	if apiToken == "" {
		return fmt.Errorf("CLOUDFLARE_API_TOKEN environment variable is not set")
	}

	zoneID := os.Getenv("CLOUDFLARE_ZONE_ID")
	if zoneID == "" {
		return fmt.Errorf("CLOUDFLARE_ZONE_ID environment variable is not set")
	}

	// Apply or destroy based on action
	if action == "create" {
		cmd := exec.Command("terraform", "apply", "-auto-approve",
			"-var", fmt.Sprintf("web_server_ip=%s", awsIP),
			"-var", fmt.Sprintf("cloudflare_api_token=%s", apiToken),
			"-var", fmt.Sprintf("cloudflare_zone_id=%s", zoneID))
		cmd.Dir = cfDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("terraform apply failed: %v", err)
		}
	} else if action == "destroy" {
		cmd := exec.Command("terraform", "destroy", "-auto-approve",
			"-var", fmt.Sprintf("web_server_ip=%s", awsIP),
			"-var", fmt.Sprintf("cloudflare_api_token=%s", apiToken),
			"-var", fmt.Sprintf("cloudflare_zone_id=%s", zoneID))
		cmd.Dir = cfDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("terraform destroy failed: %v", err)
		}
	}

	return nil
}
