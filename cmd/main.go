package main

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/cloudevents/sdk-go/pkg/cloudevents/types"
	"github.com/google/uuid"

	"github.com/keptn-contrib/argo-service/pkg/lib/argo"

	"github.com/cloudevents/sdk-go/pkg/cloudevents"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/client"
	cloudeventshttp "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/http"

	keptnutils "github.com/keptn/go-utils/pkg/lib"

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

	t, err := cloudeventshttp.New(
		cloudeventshttp.WithPort(env.Port),
		cloudeventshttp.WithPath(env.Path),
	)

	if err != nil {
		log.Fatalf("failed to create transport, %v", err)
	}
	c, err := client.New(t)
	if err != nil {
		log.Fatalf("failed to create client, %v", err)
	}

	log.Fatalf("failed to start receiver: %s", c.StartReceiver(ctx, gotEvent))

	return 0
}

func gotEvent(ctx context.Context, event cloudevents.Event) error {
	var shkeptncontext string
	event.Context.ExtensionAs("shkeptncontext", &shkeptncontext)

	logger := keptnutils.NewLogger(shkeptncontext, event.Context.GetID(), "gatekeeper-service")

	if event.Type() == keptnutils.EvaluationDoneEventType {
		go promote(event, logger)
	} else {
		logger.Error("Received unexpected keptn event")
	}

	return nil
}

func promote(event cloudevents.Event, logger *keptnutils.Logger) error {

	data := &keptnutils.EvaluationDoneEventData{}
	if err := event.DataAs(data); err != nil {
		logger.Error(fmt.Sprintf("Got Data Error: %s", err.Error()))
		return err
	}

	keptn, err := keptnutils.NewKeptn(&event, keptnutils.KeptnOpts{})
	if err != nil {
		logger.Info("failed to create keptn handler")
		return err
	}

	// Evaluation has passed if we have result = pass or result = warning
	if data.Result == "pass" || data.Result == "warning" {

		logger.Info(fmt.Sprintf("Service %s of project %s in stage %s has passed the evaluation",
			data.Service, data.Project, data.Stage))

		if strings.ToLower(data.DeploymentStrategy) == "blue_green_service" {
			// Promote rollout
			output, err := argo.Promote(data.Service+"-"+data.Stage, data.Project+"-"+data.Stage)
			if err != nil {
				logger.Error(fmt.Sprintf("Error sending promotion event "+
					"for service %s of project %s and stage %s: %s", data.Service, data.Project,
					data.Stage, err.Error()))
				return err
			}
			logger.Info(output)

			return keptn.SendCloudEvent(getCloudEvent(PromoteEventData{
				Project: data.Project,
				Service: data.Service,
				Stage:   data.Stage,
				Action:  "promote",
			}, "sh.keptn.event.release.finished", keptn.KeptnContext, event.ID()))
		}

	} else {
		logger.Info(fmt.Sprintf("Service %s of project %s in stage %s has NOT passed the evaluation",
			data.Service, data.Project, data.Stage))

		if strings.ToLower(data.DeploymentStrategy) == "blue_green_service" {
			// Abort rollout
			output, err := argo.Abort(data.Service+"-"+data.Stage, data.Project+"-"+data.Stage)
			if err != nil {
				logger.Error(fmt.Sprintf("Error sending promotion event "+
					"for service %s of project %s and stage %s: %s", data.Service, data.Project,
					data.Stage, err.Error()))
				return err
			}
			logger.Info(output)
			return keptn.SendCloudEvent(getCloudEvent(PromoteEventData{
				Project: data.Project,
				Service: data.Service,
				Stage:   data.Stage,
				Action:  "abort",
			}, "sh.keptn.event.release.finished", keptn.KeptnContext, event.ID()))
		}
	}
	return nil
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
	contentType := "application/json"

	extensions := map[string]interface{}{"shkeptncontext": shkeptncontext}
	if triggeredID != "" {
		extensions["triggeredid"] = triggeredID
	}

	return cloudevents.Event{
		Context: cloudevents.EventContextV02{
			ID:          uuid.New().String(),
			Time:        &types.Timestamp{Time: time.Now()},
			Type:        ceType,
			Source:      types.URLRef{URL: *source},
			ContentType: &contentType,
			Extensions:  extensions,
		}.AsV02(),
		Data: data,
	}
}
