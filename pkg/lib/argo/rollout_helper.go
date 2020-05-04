package argo

import (
	"fmt"
	"os/exec"
	"strings"
)

// Promote promotes a rollout
func Promote(rolloutName string, namespace string) (string, error) {
	return executeCommand("kubectl",
		[]string{"argo", "rollouts", "promote", rolloutName, "-n", namespace})
}

// Abort aborts a rollout
func Abort(rolloutName string, namespace string) (string, error) {
	return executeCommand("kubectl",
		[]string{"argo", "rollouts", "abort", rolloutName, "-n", namespace})
}

// executeCommand exectues the command using the args
func executeCommand(command string, args []string) (string, error) {
	cmd := exec.Command(command, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("Error executing command %s %s: %s\n%s", command, strings.Join(args, " "), err.Error(), string(out))
	}
	return string(out), nil
}
