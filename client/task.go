package client

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	cloudtasks "cloud.google.com/go/cloudtasks/apiv2"
	taskspb "cloud.google.com/go/cloudtasks/apiv2/cloudtaskspb"
	"github.com/andrewshostak/result-service/config"
	"github.com/andrewshostak/result-service/errs"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	checkResultQueue      = "check-result"
	notifySubscriberQueue = "notify-subscriber"
)

const (
	checkResultPath      = "/v1/triggers/result_check"
	notifySubscriberPath = "/v1/triggers/subscriber_notification"
)

const (
	errAlreadyExists = "AlreadyExists"
)

type TaskClient struct {
	client *cloudtasks.Client
	config config.GoogleCloud
}

func NewClient(config config.GoogleCloud, client *cloudtasks.Client) *TaskClient {
	return &TaskClient{config: config, client: client}
}

func (c *TaskClient) ScheduleResultCheck(ctx context.Context, matchID uint, attempt uint, scheduleAt time.Time) (*Task, error) {
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
			Name:         fmt.Sprintf("%s/tasks/match-%d-attempt-%d", queuePath, matchID, attempt),
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
		if c.isTaskAlreadyExistsError(err) {
			return nil, fmt.Errorf("result-check task already exists: %w", errs.ClientTaskAlreadyExistsError{Message: err.Error()})
		}

		return nil, fmt.Errorf("failed to create result-check task: %w", err)
	}

	return &Task{
		Name:      createdTask.Name,
		ExecuteAt: createdTask.ScheduleTime.AsTime(),
	}, nil
}

func (c *TaskClient) DeleteResultCheckTask(ctx context.Context, taskName string) error {
	req := &taskspb.DeleteTaskRequest{Name: taskName}
	err := c.client.DeleteTask(ctx, req)

	if err != nil {
		return fmt.Errorf("failed to delete result-check task: %w", err)
	}

	return nil
}

func (c *TaskClient) ScheduleSubscriberNotification(ctx context.Context, subscriptionID uint) error {
	targetURL := fmt.Sprintf("%s%s", c.config.TasksBaseURL, notifySubscriberPath)

	queuePath := fmt.Sprintf("projects/%s/locations/%s/queues/%s", c.config.ProjectID, c.config.Region, notifySubscriberQueue)

	payload := map[string]uint{"subscription_id": subscriptionID}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req := &taskspb.CreateTaskRequest{
		Parent: queuePath,
		Task: &taskspb.Task{
			Name: fmt.Sprintf("%s/tasks/subscription-%d", queuePath, subscriptionID),
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

	if _, err := c.client.CreateTask(ctx, req); err != nil {
		return fmt.Errorf("failed to create subscriber-notification task: %w", err)
	}

	return nil
}

func (c *TaskClient) isTaskAlreadyExistsError(err error) bool {
	return strings.Contains(err.Error(), errAlreadyExists)
}
