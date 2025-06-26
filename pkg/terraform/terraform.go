package terraform

import (
	"fmt"
	"os/exec"
)

func Init(dir string) error {
	cmd := exec.Command("terraform", "init")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("terraform init failed: %v\n%s", err, string(output))
	}
	return nil
}

func Apply(dir string, autoApprove bool) error {
	args := []string{"apply"}
	if autoApprove {
		args = append(args, "-auto-approve")
	}
	cmd := exec.Command("terraform", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("terraform apply failed: %v\n%s", err, string(output))
	}
	return nil
}

func Destroy(dir string, autoApprove bool) error {
	args := []string{"destroy"}
	if autoApprove {
		args = append(args, "-auto-approve")
	}
	cmd := exec.Command("terraform", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("terraform destroy failed: %v\n%s", err, string(output))
	}
	return nil
}
