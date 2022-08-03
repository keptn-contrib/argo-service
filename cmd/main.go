package main

import (
	"github.com/keptn-contrib/argo-service/handler"
	keptnevents "github.com/keptn/go-utils/pkg/lib"
	keptnv2 "github.com/keptn/go-utils/pkg/lib/v0_2_0"
	"github.com/keptn/go-utils/pkg/sdk"
	"github.com/sirupsen/logrus"
	"log"
	"os"
)

const serviceName = "argo-service"
const envVarLogLevel = "LOG_LEVEL"
const releaseTriggeredEvent = "sh.keptn.event.release.triggered"
const rollbackTriggeredEvent = "sh.keptn.event.rollback.triggered"
const testTriggeredEvent = "sh.keptn.event.test.triggered"

func main() {
	if os.Getenv(envVarLogLevel) != "" {
		logLevel, err := logrus.ParseLevel(os.Getenv(envVarLogLevel))
		if err != nil {
			logrus.WithError(err).Error("could not parse log level provided by 'LOG_LEVEL' env var")
			logrus.SetLevel(logrus.InfoLevel)
		} else {
			logrus.SetLevel(logLevel)
		}
	}

	log.Printf("Starting %s", serviceName)

	log.Fatal(sdk.NewKeptn(
		serviceName,
		sdk.WithTaskHandler(
			releaseTriggeredEvent,
			handler.NewTriggeredEventHandler(),
			releaseTriggeredFilter),
		sdk.WithTaskHandler(
			rollbackTriggeredEvent,
			handler.NewTriggeredEventHandler(),
			releaseTriggeredFilter),
		sdk.WithTaskHandler(
			testTriggeredEvent,
			handler.NewTriggeredEventHandler(),
			testTriggeredFilter),
		sdk.WithLogger(logrus.New()),
	).Start())
}

func testTriggeredFilter(keptnHandle sdk.IKeptn, event sdk.KeptnEvent) bool {
	data := &handler.TestTriggeredExtendedEventData{}
	if err := keptnv2.Decode(event.Data, data); err != nil {
		keptnHandle.Logger().Errorf("Could not parse test.triggered event: %s", err.Error())
		return false
	}

	return data.Test.TestStrategy == handler.RealUserTestStrategy
}

func releaseTriggeredFilter(keptnHandle sdk.IKeptn, event sdk.KeptnEvent) bool {
	data := &keptnv2.ReleaseTriggeredEventData{}
	if err := keptnv2.Decode(event.Data, data); err != nil {
		keptnHandle.Logger().Errorf("Could not parse release.triggered event: %s", err.Error())
		return false
	}

	deploymentStrategy, err := keptnevents.GetDeploymentStrategy(data.Deployment.DeploymentStrategy)
	if err != nil {
		keptnHandle.Logger().Errorf("Could not parse deployment strategy: %s", err.Error())
		return false
	}

	// we only support Duplicate (Blue/Green) and UserManaged (for Canary)
	if (deploymentStrategy != keptnevents.Duplicate) && (deploymentStrategy != keptnevents.UserManaged) {
		keptnHandle.Logger().Infof("Argo-service took no action as it only supports actions on deploymentStrategy duplicate (blue/green) or user_managed (canary)")
		return false
	}

	return true
}
