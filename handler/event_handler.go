package handler

import (
	"fmt"
	"github.com/keptn-contrib/argo-service/pkg/lib/argo"
	keptnv2 "github.com/keptn/go-utils/pkg/lib/v0_2_0"
	"github.com/keptn/go-utils/pkg/sdk"
	"time"
)

const minCanaryWait = 0.0
const maxCanaryWait = 3600.0 // max wait is an hour!

// TestTriggeredExtendedEventData is an extended version of TriggeredEventData including test details
type TestTriggeredExtendedEventData struct {
	keptnv2.EventData

	Test       TestTriggeredExtendedDetails `json:"test"`
	Deployment DeploymentDetails            `json:"deployment"`
}

// TestTriggeredExtendedDetails provides details about the test strategy and canary duration
type TestTriggeredExtendedDetails struct {
	// TestStrategy is the testing strategy and is defined in the shipyard
	TestStrategy       string `json:"teststrategy" jsonschema:"enum=real-user,enum=functional,enum=performance,enum=healthcheck,enum=canarywait"`
	CanaryWaitDuration string `json:"canarywaitduration"`
}

// DeploymentDetails contains information about the deployment URI and strategy
type DeploymentDetails struct {
	// DeploymentURILocal contains the local URL
	DeploymentURIsLocal []string `json:"deploymentURIsLocal"`
	// DeploymentURIPublic contains the public URL
	DeploymentURIsPublic []string `json:"deploymentURIsPublic,omitempty"`
	// DeploymentStrategy defines the used deployment strategy
	DeploymentStrategy string `json:"deploymentstrategy" jsonschema:"enum=direct,enum=blue_green_service,enum=user_managed"`
}

// RollbackTriggeredExtendedEventData is an extended version of TriggeredEventData including deployment details
type RollbackTriggeredExtendedEventData struct {
	keptnv2.EventData

	Deployment DeploymentDetails `json:"deployment"`
}

// TriggeredEventHandler handles Keptn triggered events
type TriggeredEventHandler struct {
}

// NewTriggeredEventHandler creates a new TriggeredEventHandler
func NewTriggeredEventHandler() *TriggeredEventHandler {
	return &TriggeredEventHandler{}
}

// Execute handles action.triggered events
func (g *TriggeredEventHandler) Execute(k sdk.IKeptn, event sdk.KeptnEvent) (interface{}, *sdk.Error) {
	k.Logger().Infof("Handling Event: %s of type %s", event.ID, *event.Type)

	// Now lets see which event we want to handle
	// Release: We promote for successful evaluations or abort for failed evaluations
	// Rollback: We will send an abort
	if *event.Type == keptnv2.GetTriggeredEventType(keptnv2.ReleaseTaskName) {
		data := &keptnv2.ReleaseTriggeredEventData{}
		if err := keptnv2.Decode(event.Data, data); err != nil {
			return nil, &sdk.Error{Err: err, StatusType: keptnv2.StatusErrored, ResultType: keptnv2.ResultFailed, Message: "failed to decode release.triggered event: " + err.Error()}
		}

		return promote(k, data)
	} else if *event.Type == keptnv2.GetTriggeredEventType(keptnv2.RollbackTaskName) {

		data := &RollbackTriggeredExtendedEventData{}
		if err := keptnv2.Decode(event.Data, data); err != nil {
			return nil, &sdk.Error{Err: err, StatusType: keptnv2.StatusErrored, ResultType: keptnv2.ResultFailed, Message: "failed to decode rollback.triggered event: " + err.Error()}
		}

		return abort(k, data)
	} else if *event.Type == keptnv2.GetTriggeredEventType(keptnv2.TestTaskName) {
		data := &TestTriggeredExtendedEventData{}
		if err := keptnv2.Decode(event.Data, data); err != nil {
			return nil, &sdk.Error{Err: err, StatusType: keptnv2.StatusErrored, ResultType: keptnv2.ResultFailed, Message: "failed to decode test.triggered event: " + err.Error()}
		}

		return testCanaryWait(k, data)
	}

	return nil, nil
}

func getActionFinishedEvent(result keptnv2.ResultType, status keptnv2.StatusType, actionTriggeredEvent RollbackTriggeredExtendedEventData, message string) keptnv2.ActionFinishedEventData {

	return keptnv2.ActionFinishedEventData{
		EventData: keptnv2.EventData{
			Project: actionTriggeredEvent.Project,
			Stage:   actionTriggeredEvent.Stage,
			Service: actionTriggeredEvent.Service,
			Labels:  actionTriggeredEvent.Labels,
			Status:  status,
			Result:  result,
			Message: message,
		},
	}
}

func getReleaseFinishedEvent(result keptnv2.ResultType, status keptnv2.StatusType, releaseTriggeredEvent keptnv2.ReleaseTriggeredEventData, message string) keptnv2.ReleaseFinishedEventData {

	return keptnv2.ReleaseFinishedEventData{
		EventData: keptnv2.EventData{
			Project: releaseTriggeredEvent.Project,
			Stage:   releaseTriggeredEvent.Stage,
			Service: releaseTriggeredEvent.Service,
			Labels:  releaseTriggeredEvent.Labels,
			Status:  status,
			Result:  result,
			Message: message,
		},
	}
}

func getTestFinishedEvent(result keptnv2.ResultType, status keptnv2.StatusType, testTriggeredEvent TestTriggeredExtendedEventData, startedAt time.Time, message string) keptnv2.TestFinishedEventData {

	return keptnv2.TestFinishedEventData{
		EventData: keptnv2.EventData{
			Project: testTriggeredEvent.Project,
			Stage:   testTriggeredEvent.Stage,
			Service: testTriggeredEvent.Service,
			Labels:  testTriggeredEvent.Labels,
			Status:  status,
			Result:  result,
			Message: message,
		},
		Test: keptnv2.TestFinishedDetails{
			Start: startedAt.Format(time.RFC3339),
			End:   time.Now().Format(time.RFC3339),
		},
	}
}

func promote(k sdk.IKeptn, data *keptnv2.ReleaseTriggeredEventData) (interface{}, *sdk.Error) {
	k.Logger().Info(fmt.Sprintf("Service %s of project %s in stage %s has passed the evaluation",
		data.Service, data.Project, data.Stage))

	// Promote rollout
	output, err := argo.Promote(data.Service+"-"+data.Stage, data.Project+"-"+data.Stage)
	if err != nil {
		msg := fmt.Sprintf("Error sending rollout promotion event "+
			"for service %s of project %s and stage %s: %s", data.Service, data.Project,
			data.Stage, err.Error())
		return nil, &sdk.Error{Err: err, StatusType: keptnv2.StatusErrored, ResultType: keptnv2.ResultFailed, Message: msg}
	}
	k.Logger().Info(output)
	k.Logger().Infof("Successfully sent promotion event "+
		"for service %s of project %s and stage %s: %s", data.Service, data.Project, data.Stage, output)

	finishedEventData := getReleaseFinishedEvent(keptnv2.ResultPass, keptnv2.StatusSucceeded, *data, "")

	return finishedEventData, nil
}

/**
 * Waits for the canaryWaitSeconds
 */
func testCanaryWait(k sdk.IKeptn, data *TestTriggeredExtendedEventData) (interface{}, *sdk.Error) {
	// lets parse the canarywaitduration
	waitDuration, err := time.ParseDuration(data.Test.CanaryWaitDuration)
	if err != nil {
		return nil, &sdk.Error{Err: err, StatusType: keptnv2.StatusErrored, ResultType: keptnv2.ResultFailed, Message: fmt.Sprintf("Didn't wait because CanaryWaitSeconds of %s is invalid! %s", data.Test.CanaryWaitDuration, err)}
	}

	// lets wait for the defined seconds
	if (waitDuration.Seconds() >= minCanaryWait) && (waitDuration.Seconds() < maxCanaryWait) {
		startedAt := time.Now()

		// lets wait
		<-time.After(waitDuration)

		finishedEventData := getTestFinishedEvent(keptnv2.ResultPass, keptnv2.StatusSucceeded, *data, startedAt, fmt.Sprintf("Successfully waited for %s seconds!", data.Test.CanaryWaitDuration))
		return finishedEventData, nil
	}

	return nil, &sdk.Error{Err: err, StatusType: keptnv2.StatusErrored, ResultType: keptnv2.ResultFailed, Message: fmt.Sprintf("Didn't wait because CanaryWaitSeconds (%s:%f) not within MIN=%f & MAX=%f!", data.Test.CanaryWaitDuration, waitDuration.Seconds(), minCanaryWait, maxCanaryWait)}
}

func abort(k sdk.IKeptn, data *RollbackTriggeredExtendedEventData) (interface{}, *sdk.Error) {
	// Abort rollout
	output, err := argo.Abort(data.Service+"-"+data.Stage, data.Project+"-"+data.Stage)
	if err != nil {
		msg := fmt.Sprintf("Error sending abort event for service %s of project %s and stage %s: %s", data.Service, data.Project,
			data.Stage, err.Error())
		k.Logger().Error(msg)

		return nil, &sdk.Error{Err: err, StatusType: keptnv2.StatusErrored, ResultType: keptnv2.ResultFailed, Message: msg}
	}

	k.Logger().Info(output)

	msg := fmt.Sprintf("Successfully sent abort event for service %s of project %s and stage %s: %s", data.Service, data.Project, data.Stage, output)
	k.Logger().Info(msg)

	finishedEventData := getActionFinishedEvent(keptnv2.ResultPass, keptnv2.StatusSucceeded, *data, msg)
	return finishedEventData, nil
}
