package argo

import (
	"fmt"
	"os/exec"
	"strings"
)

// Promote promotes a rollout
func Promote(rolloutName string, namespace string) (string, error) {
	// first execute kubectl argo rollouts promote rolloutName -n namespace
	output, err := executeCommand("kubectl",
		[]string{"argo", "rollouts", "promote", rolloutName, "-n", namespace})

	if err != nil {
		return output, err
	}

	// now lets also capture the current status of the rollout so we can add it to the output
	outputRollout, err2 := executeCommand("kubectl",
		[]string{"argo", "rollouts", "get", "rollout", rolloutName, "-n", namespace})

	if err2 != nil {
		output = fmt.Sprintf("%s\nCouldn't retrieve rollout overview! %s", err2.Error())
		return output, nil
	}

	combinedOutput := fmt.Sprintf("%s\n%s", output, outputRollout)
	return combinedOutput, nil
}

// Abort aborts a rollout
func Abort(rolloutName string, namespace string) (string, error) {
	// first execute kubectl argo rollouts abort rolloutName -n namespace
	output, err := executeCommand("kubectl",
		[]string{"argo", "rollouts", "abort", rolloutName, "-n", namespace})

	if err != nil {
		return output, err
	}

	// now lets also capture the current status of the rollout so we can add it to the output
	outputRollout, err2 := executeCommand("kubectl",
		[]string{"argo", "rollouts", "get", "rollout", rolloutName, "-n", namespace})

	if err2 != nil {
		output = fmt.Sprintf("%s\nCouldn't retrieve rollout overview! %s", err2.Error())
		return output, nil
	}

	combinedOutput := fmt.Sprintf("%s\n%s", output, outputRollout)
	return combinedOutput, nil

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
