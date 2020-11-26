package main

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"time"

	"github.com/google/uuid"

	"github.com/keptn-contrib/argo-service/pkg/lib/argo"

	cloudevents "github.com/cloudevents/sdk-go/v2"

	keptnevents "github.com/keptn/go-utils/pkg/lib"
	keptnutils "github.com/keptn/go-utils/pkg/lib/keptn"
	keptnv2 "github.com/keptn/go-utils/pkg/lib/v0_2_0"

	"github.com/kelseyhightower/envconfig"
)

type envConfig struct {
	// Port on which to listen for cloudevents
	Port int    `envconfig:"RCV_PORT" default:"8080"`
	Path string `envconfig:"RCV_PATH" default:"/"`
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

func gotEvent(ctx context.Context, event cloudevents.Event) error {
	var shkeptncontext string
	event.Context.ExtensionAs("shkeptncontext", &shkeptncontext)

	logger := keptnutils.NewLogger(shkeptncontext, event.Context.GetID(), "gatekeeper-service")

	if event.Type() == keptnv2.GetTriggeredEventType(keptnv2.ReleaseTaskName) {
		go promote(event, logger)
	} else {
		logger.Error("Received unexpected keptn event")
	}

	return nil
}

func promote(event cloudevents.Event, logger *keptnutils.Logger) error {

	data := &keptnv2.ReleaseTriggeredEventData{}
	if err := event.DataAs(data); err != nil {
		logger.Error(fmt.Sprintf("Got Data Error: %s", err.Error()))
		return err
	}

	keptn, err := keptnv2.NewKeptn(&event, keptnutils.KeptnOpts{})
	if err != nil {
		logger.Info("failed to create keptn handler")
		return err
	}

	if err := sendReleaseStartedEvent(event, keptn, data); err != nil {
		msg := fmt.Sprintf("Error sending release.started event "+
			"for service %s of project %s and stage %s: %s", data.Service, data.Project,
			data.Stage, err.Error())
		logger.Error(msg)
		return sendReleaseFailedFinishedEvent(event, keptn, data, msg)
	}

	deploymentStrategy, err := keptnevents.GetDeploymentStrategy(data.Deployment.DeploymentStrategy)
	if err != nil {
		msg := fmt.Sprintf("Error determining deployment strategy "+
			"for service %s of project %s and stage %s: %s", data.Service, data.Project,
			data.Stage, err.Error())
		logger.Error(msg)
		return sendReleaseFailedFinishedEvent(event, keptn, data, msg)
	}

	// Evaluation has passed if we have result = pass or result = warning
	if data.Result == keptnv2.ResultPass || data.Result == keptnv2.ResultWarning {

		logger.Info(fmt.Sprintf("Service %s of project %s in stage %s has passed the evaluation",
			data.Service, data.Project, data.Stage))

		if deploymentStrategy == keptnevents.Duplicate {
			// Promote rollout
			output, err := argo.Promote(data.Service+"-"+data.Stage, data.Project+"-"+data.Stage)
			if err != nil {
				msg := fmt.Sprintf("Error sending promotion event "+
					"for service %s of project %s and stage %s: %s", data.Service, data.Project,
					data.Stage, err.Error())
				logger.Error(msg)
				return sendReleaseFailedFinishedEvent(event, keptn, data, msg)
			}
			logger.Info(output)

			return sendReleaseSucceededFinishedEvent(event, keptn, data)
		}
	} else {
		logger.Info(fmt.Sprintf("Service %s of project %s in stage %s has NOT passed the evaluation",
			data.Service, data.Project, data.Stage))

		if deploymentStrategy == keptnevents.Duplicate {
			// Abort rollout
			output, err := argo.Abort(data.Service+"-"+data.Stage, data.Project+"-"+data.Stage)
			if err != nil {
				msg := fmt.Sprintf("Error sending promotion event "+
					"for service %s of project %s and stage %s: %s", data.Service, data.Project,
					data.Stage, err.Error())
				logger.Error(msg)
				return sendReleaseFailedFinishedEvent(event, keptn, data, msg)
			}
			logger.Info(output)
			return sendReleaseSucceededFinishedEvent(event, keptn, data)
		}
	}
	return nil
}

func sendReleaseSucceededFinishedEvent(event cloudevents.Event, keptn *keptnv2.Keptn, data *keptnv2.ReleaseTriggeredEventData) error {
	return keptn.SendCloudEvent(getCloudEvent(keptnv2.ReleaseFinishedEventData{
		EventData: keptnv2.EventData{
			Project: data.Project,
			Stage:   data.Stage,
			Service: data.Service,
			Labels:  data.Labels,
			Status:  keptnv2.StatusSucceeded,
			Result:  keptnv2.ResultPass,
		},
		Release: keptnv2.ReleaseData{},
	}, keptnv2.GetFinishedEventType(keptnv2.ReleaseTaskName), keptn.KeptnContext, event.ID()))
}

func sendReleaseFailedFinishedEvent(event cloudevents.Event, keptn *keptnv2.Keptn, data *keptnv2.ReleaseTriggeredEventData, msg string) error {
	return keptn.SendCloudEvent(getCloudEvent(keptnv2.ReleaseFinishedEventData{
		EventData: keptnv2.EventData{
			Project: data.Project,
			Stage:   data.Stage,
			Service: data.Service,
			Labels:  data.Labels,
			Status:  keptnv2.StatusErrored,
			Result:  keptnv2.ResultFailed,
			Message: msg,
		},
		Release: keptnv2.ReleaseData{},
	}, keptnv2.GetFinishedEventType(keptnv2.ReleaseTaskName), keptn.KeptnContext, event.ID()))
}

func sendReleaseStartedEvent(event cloudevents.Event, keptn *keptnv2.Keptn, data *keptnv2.ReleaseTriggeredEventData) error {
	return keptn.SendCloudEvent(getCloudEvent(keptnv2.ReleaseFinishedEventData{
		EventData: keptnv2.EventData{
			Project: data.Project,
			Stage:   data.Stage,
			Service: data.Service,
			Labels:  data.Labels,
		},
	}, keptnv2.GetStartedEventType(keptnv2.ReleaseTaskName), keptn.KeptnContext, event.ID()))
}

// ConfigurationChangeEventData represents the data for changing the service configuration
type PromoteEventData struct {
	// Project is the name of the project
	Project string `json:"project"`
	// Service is the name of the new service
	Service string `json:"service"`
	// Stage is the name of the stage
	Stage string `json:"stage"`

	Action string `json:action`
}

func getCloudEvent(data interface{}, ceType string, shkeptncontext string, triggeredID string) cloudevents.Event {

	source, _ := url.Parse("argo-service")

	extensions := map[string]interface{}{"shkeptncontext": shkeptncontext}
	if triggeredID != "" {
		extensions["triggeredid"] = triggeredID
	}

	event := cloudevents.NewEvent()
	event.SetID(uuid.New().String())
	event.SetTime(time.Now())
	event.SetType(ceType)
	event.SetSource(source.String())
	event.SetDataContentType(cloudevents.ApplicationJSON)
	event.SetData(cloudevents.ApplicationJSON, data)

	return event
}
