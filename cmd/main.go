package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/keptn-contrib/argo-service/pkg/lib/argo"

	"github.com/cloudevents/sdk-go/pkg/cloudevents"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/client"
	cloudeventshttp "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/http"

	keptnevents "github.com/keptn/go-utils/pkg/events"
	keptnutils "github.com/keptn/go-utils/pkg/utils"

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

	if event.Type() == keptnevents.EvaluationDoneEventType {
		go promote(event, logger)
	} else {
		logger.Error("Received unexpected keptn event")
	}

	return nil
}

func promote(event cloudevents.Event, logger *keptnutils.Logger) error {

	data := &keptnevents.EvaluationDoneEventData{}
	if err := event.DataAs(data); err != nil {
		logger.Error(fmt.Sprintf("Got Data Error: %s", err.Error()))
		return err
	}

	// Evaluation has passed if we have result = pass or result = warning
	if data.Result == "pass" || data.Result == "warning" {

		logger.Info(fmt.Sprintf("Service %s of project %s in stage %s has passed the evaluation",
			data.Service, data.Project, data.Stage))

		if strings.ToLower(data.DeploymentStrategy) == "blue_green_service" {
			// Promote rollout
			if err := argo.Promote(data.Service+"-"+data.Stage, data.Project+"-"+data.Stage); err != nil {
				logger.Error(fmt.Sprintf("Error sending promotion event "+
					"for service %s of project %s and stage %s: %s", data.Service, data.Project,
					data.Stage, err.Error()))
				return err
			}
		}

	} else {
		logger.Info(fmt.Sprintf("Service %s of project %s in stage %s has NOT passed the evaluation",
			data.Service, data.Project, data.Stage))

		if strings.ToLower(data.DeploymentStrategy) == "blue_green_service" {
			// Abort rollout
			if err := argo.Abort(data.Service+"-"+data.Stage, data.Project+"-"+data.Stage); err != nil {
				logger.Error(fmt.Sprintf("Error sending promotion event "+
					"for service %s of project %s and stage %s: %s", data.Service, data.Project,
					data.Stage, err.Error()))
				return err
			}
		}
	}
	return nil
}
