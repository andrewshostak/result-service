package client

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	cloudtasks "cloud.google.com/go/cloudtasks/apiv2"
	taskspb "cloud.google.com/go/cloudtasks/apiv2/cloudtaskspb"
	"github.com/andrewshostak/result-service/config"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	checkResultQueue      = "check-result"
	notifySubscriberQueue = "notify-subscriber"
)

const (
	checkResultPath = "/v1/triggers/result_check"
)

type TaskClient struct {
	client *cloudtasks.Client
	config config.GoogleCloud
}

func NewClient(config config.GoogleCloud, client *cloudtasks.Client) *TaskClient {
	return &TaskClient{config: config, client: client}
}

func (c *TaskClient) ScheduleResultCheck(ctx context.Context, matchID uint, scheduleAt time.Time) (*string, error) {
	// targetURL := "https://webhook.site/afe7dfa2-5520-4ab2-88c2-16188835e41d" // TODO
	targetURL := fmt.Sprintf("%s%s", c.config.TasksBaseURL, checkResultPath)

	queuePath := fmt.Sprintf("projects/%s/locations/%s/queues/%s", c.config.ProjectID, c.config.Region, checkResultQueue)

	payload := map[string]uint{"match_id": matchID}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req := &taskspb.CreateTaskRequest{
		Parent: queuePath,
		Task: &taskspb.Task{
			ScheduleTime: timestamppb.New(scheduleAt),
			MessageType: &taskspb.Task_HttpRequest{
				HttpRequest: &taskspb.HttpRequest{
					HttpMethod: taskspb.HttpMethod_POST,
					Url:        targetURL,
					Body:       body,
					Headers: map[string]string{
						"Content-Type": "application/json",
					},
				},
			},
		},
	}

	createdTask, err := c.client.CreateTask(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create result-check task: %w", err)
	}

	return &createdTask.Name, nil
}

func (c *TaskClient) ScheduleSubscriberNotification(ctx context.Context, subscriptionID uint) error {
	panic("implement me")
}
