package argo

import (
	keptnutils "github.com/keptn/go-utils/pkg/utils"
)

// Promote promotes a rollout
func Promote(rolloutName string, namespace string) error {
	_, err := keptnutils.ExecuteCommand("kubectl",
		[]string{"argo", "rollouts", "promote", rolloutName, "-n", namespace})
	return err
}

// Abort aborts a rollout
func Abort(rolloutName string, namespace string) error {
	_, err := keptnutils.ExecuteCommand("kubectl",
		[]string{"argo", "rollouts", "abort", rolloutName, "-n", namespace})
	return err
}
