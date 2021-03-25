package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/keptn-contrib/argo-service/pkg/lib/argo"

	cloudevents "github.com/cloudevents/sdk-go/v2"

	keptnevents "github.com/keptn/go-utils/pkg/lib"
	keptnutils "github.com/keptn/go-utils/pkg/lib/keptn"
	keptnv2 "github.com/keptn/go-utils/pkg/lib/v0_2_0"

	"github.com/kelseyhightower/envconfig"
)

const ServiceName = "argo-service"
const RealUserTestStrategy = "real-user"
const MINCANARYWAIT = 0.0
const MAXCANARYWAIT = 3600.0 // max wait is an hour!

type envConfig struct {
	// Port on which to listen for cloudevents
	Port int    `envconfig:"RCV_PORT" default:"8080"`
	Path string `envconfig:"RCV_PATH" default:"/"`
}

/**
 * This is an extended version of the Release TriggeredEventData including a new section for
 */

type TestTriggeredExtendedEventData struct {
	keptnv2.EventData

	Test       TestTriggeredExtendedDetails `json:"test"`
	Deployment DeploymentDetails            `json:"deployment"`
}

type DeploymentDetails struct {
	// DeploymentURILocal contains the local URL
	DeploymentURIsLocal []string `json:"deploymentURIsLocal"`
	// DeploymentURIPublic contains the public URL
	DeploymentURIsPublic []string `json:"deploymentURIsPublic,omitempty"`
	// DeploymentStrategy defines the used deployment strategy
	DeploymentStrategy string `json:"deploymentstrategy" jsonschema:"enum=direct,enum=blue_green_service,enum=user_managed"`
}

type TestTriggeredExtendedDetails struct {
	// TestStrategy is the testing strategy and is defined in the shipyard
	TestStrategy       string `json:"teststrategy" jsonschema:"enum=real-user,enum=functional,enum=performance,enum=healthcheck,enum=canarywait"`
	CanaryWaitDuration string `json:"canarywaitduration"`
}

type RollbackTriggeredExtendedEventData struct {
	keptnv2.EventData

	Deployment DeploymentDetails `json:"deployment"`
}

func main() {
	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		log.Fatalf("Failed to process env var: %s", err)
	}
	os.Exit(_main(os.Args[1:], env))
}

func _main(args []string, env envConfig) int {

	ctx := context.Background()
	ctx = cloudevents.WithEncodingStructured(ctx)

	p, err := cloudevents.NewHTTP(cloudevents.WithPath(env.Path), cloudevents.WithPort(env.Port))
	if err != nil {
		log.Fatalf("failed to create client, %v", err)
	}
	c, err := cloudevents.NewClient(p)
	if err != nil {
		log.Fatalf("failed to create client, %v", err)
	}
	log.Fatal(c.StartReceiver(ctx, gotEvent))

	return 0
}

/**
 * Handles incoming events
 */
func gotEvent(ctx context.Context, event cloudevents.Event) error {
	var shkeptncontext string
	event.Context.ExtensionAs("shkeptncontext", &shkeptncontext)

	logger := keptnutils.NewLogger(shkeptncontext, event.Context.GetID(), ServiceName)
	myKeptn, err := keptnv2.NewKeptn(&event, keptnutils.KeptnOpts{})
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to create keptn handler: %v", err))
		return err
	}

	logger.Info(fmt.Sprintf("gotEvent(%s): %s - %s", event.Type(), myKeptn.KeptnContext, event.Context.GetID()))

	// Now lets see which event we want to handle
	// Release: We promote for successful evaluations or abort for failed evaluations
	// Rollback: We will send an abort
	if event.Type() == keptnv2.GetTriggeredEventType(keptnv2.ReleaseTaskName) {
		data := &keptnv2.ReleaseTriggeredEventData{}
		if err := event.DataAs(data); err != nil {
			logger.Error(fmt.Sprintf("Got GetTriggeredEventType Error: %s", err.Error()))
			return err
		}

		go promote(myKeptn, event, data, logger)
	} else if event.Type() == keptnv2.GetTriggeredEventType(keptnv2.RollbackTaskName) {

		data := &RollbackTriggeredExtendedEventData{}
		if err := event.DataAs(data); err != nil {
			logger.Error(fmt.Sprintf("Got RollbackTriggeredExtendedEventData Error: %s", err.Error()))
			return err
		}

		go abort(myKeptn, event, data, logger)
	} else if event.Type() == keptnv2.GetTriggeredEventType(keptnv2.TestTaskName) {
		data := &TestTriggeredExtendedEventData{}
		if err := event.DataAs(data); err != nil {
			logger.Error(fmt.Sprintf("Got TestTriggeredExtendedEventData Error: %s", err.Error()))
			return err
		}

		// only handle canarywait
		if data.Test.TestStrategy == RealUserTestStrategy {
			go testCanaryWait(myKeptn, event, data, logger)
		} else {
			logger.Info(fmt.Sprintf("No doing anything as teststrategy is %s. We just wait on %s", data.Test.TestStrategy, RealUserTestStrategy))
		}

	} else {
		logger.Error("Received unexpected keptn event")
	}

	return nil
}

/**
 * Waits for the canaryWaitSeconds
 */
func testCanaryWait(myKeptn *keptnv2.Keptn, incomingEvent cloudevents.Event, data *TestTriggeredExtendedEventData, logger *keptnutils.Logger) error {

	// lets send test.started
	_, err := myKeptn.SendTaskStartedEvent(data, ServiceName)
	if err != nil {
		msg := fmt.Sprintf("Error sending test.started event for %s on service %s of project %s and stage %s: %s",
			RealUserTestStrategy, data.Service, data.Project, data.Stage, err.Error())
		logger.Error(msg)
		return err
	}

	// lets parse the canarywaitduration
	waitDuration, err := time.ParseDuration(data.Test.CanaryWaitDuration)
	if err != nil {
		_, err = myKeptn.SendTaskFinishedEvent(&keptnv2.EventData{
			Status:  keptnv2.StatusErrored,
			Result:  keptnv2.ResultFailed,
			Message: fmt.Sprintf("Didn't wait because CanaryWaitSeconds of %s is invalid! %s", data.Test.CanaryWaitDuration, err),
		}, ServiceName)
		if err != nil {
			msg := fmt.Sprintf("Error sending test.finished event for service %s of project %s and stage %s: %s",
				data.Service, data.Project, data.Stage, err.Error())
			logger.Error(msg)
			return err
		}

		return nil
	}

	// lets wait for the defined seconds
	if (waitDuration.Seconds() >= MINCANARYWAIT) && (waitDuration.Seconds() < MAXCANARYWAIT) {
		startedAt := time.Now()

		// lets wait
		<-time.After(waitDuration)

		// lets send the test finished event
		testFinishedEventData := &keptnv2.TestFinishedEventData{
			EventData: keptnv2.EventData{
				Status:  keptnv2.StatusSucceeded,
				Result:  keptnv2.ResultPass,
				Message: fmt.Sprintf("Successfully waited for %s seconds!", data.Test.CanaryWaitDuration),
			},
			Test: keptnv2.TestFinishedDetails{
				Start: startedAt.Format(time.RFC3339),
				End:   time.Now().Format(time.RFC3339),
			},
		}

		_, err := myKeptn.SendTaskFinishedEvent(testFinishedEventData, ServiceName)
		if err != nil {
			msg := fmt.Sprintf("Error sending test.finished event for service %s of project %s and stage %s: %s",
				data.Service, data.Project, data.Stage, err.Error())
			logger.Error(msg)
			return err
		}

		return nil
	}

	// in this case we got an incorrect wait time and send a failure event
	_, err = myKeptn.SendTaskFinishedEvent(&keptnv2.EventData{
		Status:  keptnv2.StatusErrored,
		Result:  keptnv2.ResultFailed,
		Message: fmt.Sprintf("Didn't wait because CanaryWaitSeconds (%s:%f) not within MIN=%f & MAX=%f!", data.Test.CanaryWaitDuration, waitDuration.Seconds(), MINCANARYWAIT, MAXCANARYWAIT),
	}, ServiceName)
	if err != nil {
		msg := fmt.Sprintf("Error sending test.finished event for service %s of project %s and stage %s: %s",
			data.Service, data.Project, data.Stage, err.Error())
		logger.Error(msg)
		return err
	}

	return nil
}

func promote(myKeptn *keptnv2.Keptn, incomingEvent cloudevents.Event, data *keptnv2.ReleaseTriggeredEventData, logger *keptnutils.Logger) error {

	_, err := myKeptn.SendTaskStartedEvent(data, ServiceName)
	if err != nil {
		msg := fmt.Sprintf("Error sending release.started event on service %s of project %s and stage %s: %s",
			data.Service, data.Project, data.Stage, err.Error())
		logger.Error(msg)
		return err
	}

	// lets see whether we support the passed deployment strategy
	// we either allow empty
	deploymentStrategy := keptnevents.UserManaged
	if data.Deployment.DeploymentStrategy != "" {
		deploymentStrategy, err = keptnevents.GetDeploymentStrategy(data.Deployment.DeploymentStrategy)
		if err != nil {
			msg := fmt.Sprintf("Error determining deployment strategy from %s "+
				"for service %s of project %s and stage %s: %s", data.Deployment.DeploymentStrategy, data.Service, data.Project,
				data.Stage, err.Error())
			logger.Error(msg)

			return sendReleaseFailedFinishedEvent(myKeptn, incomingEvent, data, logger, msg)
		}
	}

	// we only support Duplicate (Blue/Green) and UserManaged (for Canary)
	if (deploymentStrategy != keptnevents.Duplicate) && (deploymentStrategy != keptnevents.UserManaged) {
		msg := "Argo-service took no action as it only supports actions on deploymentStrategy duplicate (blue/green) or user_managed (canary)"
		return sendReleaseSucceededFinishedEvent(myKeptn, incomingEvent, data, logger, msg)
	}

	logger.Info(fmt.Sprintf("Service %s of project %s in stage %s has passed the evaluation",
		data.Service, data.Project, data.Stage))

	// Promote rollout
	output, err := argo.Promote(data.Service+"-"+data.Stage, data.Project+"-"+data.Stage)
	if err != nil {
		msg := fmt.Sprintf("Error sending rollout promotion event "+
			"for service %s of project %s and stage %s: %s", data.Service, data.Project,
			data.Stage, err.Error())
		return sendReleaseFailedFinishedEvent(myKeptn, incomingEvent, data, logger, msg)
	}
	logger.Info(output)
	output = formatMessageForBridgeOutput(output)

	msg := fmt.Sprintf("Successfully sent promotion event "+
		"for service %s of project %s and stage %s: %s", data.Service, data.Project, data.Stage, output)

	return sendReleaseSucceededFinishedEvent(myKeptn, incomingEvent, data, logger, msg)

	/*	// Evaluation has passed if we have result = pass or result = warning
		if data.Result == keptnv2.ResultPass || data.Result == keptnv2.ResultWarning {

		}

		// if not passed
		logger.Info(fmt.Sprintf("Service %s of project %s in stage %s has NOT passed the evaluation. Therefore we are ABORTING the promotion!!",
			data.Service, data.Project, data.Stage))

		// Abort rollout
		output, err := argo.Abort(data.Service+"-"+data.Stage, data.Project+"-"+data.Stage)
		if err != nil {
			msg := fmt.Sprintf("Error sending abort event "+
				"for service %s of project %s and stage %s because evaluation was NOT PASSED: %s", data.Service, data.Project,
				data.Stage, err.Error())
			logger.Error(msg)
			return sendReleaseFailedFinishedEvent(myKeptn, incomingEvent, data, logger, msg)
		}
		logger.Info(output)
		output = formatMessageForBridgeOutput(output)

		msg := fmt.Sprintf("Successfully sent abort event "+
			"for service %s of project %s and stage %s because evaluation was NOT PASSED: %s", data.Service, data.Project, data.Stage, output)
		return sendReleaseSucceededFinishedEvent(myKeptn, incomingEvent, data, logger, msg)*/
}

func abort(myKeptn *keptnv2.Keptn, incomingEvent cloudevents.Event, data *RollbackTriggeredExtendedEventData, logger *keptnutils.Logger) error {

	/**
	 * Only acting if we have an Argo Rollout
	 * -
	 */

	deploymentStrategy, err := keptnevents.GetDeploymentStrategy(data.Deployment.DeploymentStrategy)

	// we only support Duplicate (Blue/Green) and UserManaged (for Canary)
	if (deploymentStrategy != keptnevents.Duplicate) && (deploymentStrategy != keptnevents.UserManaged) {
		msg := "Argo-service took no rollback action as it only supports actions on deploymentStrategy duplicate (blue/green) or user_managed (canary)"
		logger.Info(msg)
		return nil
	}

	_, err = myKeptn.SendTaskStartedEvent(data, ServiceName)
	if err != nil {
		msg := fmt.Sprintf("Error sending rollback.started event on service %s of project %s and stage %s: %s",
			data.Service, data.Project, data.Stage, err.Error())
		logger.Error(msg)
		return err
	}

	// Abort rollout
	output, err := argo.Abort(data.Service+"-"+data.Stage, data.Project+"-"+data.Stage)
	if err != nil {
		msg := fmt.Sprintf("Error sending abort event for service %s of project %s and stage %s: %s", data.Service, data.Project,
			data.Stage, err.Error())
		logger.Error(msg)
		_, err = myKeptn.SendTaskFinishedEvent(&keptnv2.EventData{
			Status:  keptnv2.StatusErrored,
			Result:  keptnv2.ResultFailed,
			Message: msg,
		}, ServiceName)
		return err
	}

	logger.Info(output)
	output = formatMessageForBridgeOutput(output)

	msg := fmt.Sprintf("Successfully sent abort event for service %s of project %s and stage %s: %s", data.Service, data.Project, data.Stage, output)
	_, err = myKeptn.SendTaskFinishedEvent(&keptnv2.EventData{
		Status:  keptnv2.StatusSucceeded,
		Result:  keptnv2.ResultPass,
		Message: msg,
	}, ServiceName)

	return err
}

func sendReleaseSucceededFinishedEvent(myKeptn *keptnv2.Keptn, incomingEvent cloudevents.Event, data *keptnv2.ReleaseTriggeredEventData, logger *keptnutils.Logger, msg string) error {

	logger.Info(msg)
	_, err := myKeptn.SendTaskFinishedEvent(&keptnv2.EventData{
		Status:  keptnv2.StatusSucceeded,
		Result:  keptnv2.ResultPass,
		Message: msg,
	}, ServiceName)

	return err
}

func sendReleaseFailedFinishedEvent(myKeptn *keptnv2.Keptn, incomingEvent cloudevents.Event, data *keptnv2.ReleaseTriggeredEventData, logger *keptnutils.Logger, msg string) error {

	logger.Error(msg)
	_, err := myKeptn.SendTaskFinishedEvent(&keptnv2.EventData{
		Status:  keptnv2.StatusErrored,
		Result:  keptnv2.ResultFailed,
		Message: msg,
	}, ServiceName)

	return err
}

func formatMessageForBridgeOutput(msg string) string {
	// for now we just replace linefeeds with <br>
	// return strings.ReplaceAll(msg, "\n", "<br>")
	return msg
}
