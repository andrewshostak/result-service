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
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	checkResultPath      = "/v1/triggers/result_check"
	notifySubscriberPath = "/v1/triggers/subscriber_notification"
)

const (
	errAlreadyExists = "AlreadyExists"
)

type TaskClient struct {
	client           *cloudtasks.Client
	config           config.GoogleCloud
	dispatchDeadline time.Duration
}

func NewClient(config config.GoogleCloud, dispatchDeadline time.Duration, client *cloudtasks.Client) *TaskClient {
	return &TaskClient{config: config, dispatchDeadline: dispatchDeadline, client: client}
}

func (c *TaskClient) GetResultCheckTask(ctx context.Context, matchID uint, attempt uint) (*Task, error) {
	queuePath := fmt.Sprintf("projects/%s/locations/%s/queues/%s", c.config.ProjectID, c.config.Region, c.config.CheckResultQueueName)
	name := fmt.Sprintf("%s/tasks/match-%d-attempt-%d", queuePath, matchID, attempt)

	req := &taskspb.GetTaskRequest{Name: name}

	task, err := c.client.GetTask(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get result-check task: %w", err)
	}

	return &Task{Name: task.Name, ExecuteAt: task.ScheduleTime.AsTime()}, nil
}

func (c *TaskClient) ScheduleResultCheck(ctx context.Context, matchID uint, attempt uint, scheduleAt time.Time) (*Task, error) {
	targetURL := fmt.Sprintf("%s%s", c.config.TasksBaseURL, checkResultPath)

	queuePath := fmt.Sprintf("projects/%s/locations/%s/queues/%s", c.config.ProjectID, c.config.Region, c.config.CheckResultQueueName)

	payload := map[string]uint{"match_id": matchID}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req := &taskspb.CreateTaskRequest{
		Parent: queuePath,
		Task: &taskspb.Task{
			Name:             fmt.Sprintf("%s/tasks/match-%d-attempt-%d", queuePath, matchID, attempt),
			ScheduleTime:     timestamppb.New(scheduleAt),
			DispatchDeadline: durationpb.New(c.dispatchDeadline),
			MessageType: &taskspb.Task_HttpRequest{
				HttpRequest: &taskspb.HttpRequest{
					HttpMethod: taskspb.HttpMethod_POST,
					Url:        targetURL,
					Body:       body,
					Headers: map[string]string{
						"Content-Type": "application/json",
					},
					AuthorizationHeader: &taskspb.HttpRequest_OidcToken{
						OidcToken: &taskspb.OidcToken{
							ServiceAccountEmail: c.config.ServiceAccountEmail,
							Audience:            c.config.TasksBaseURL,
						},
					},
				},
			},
		},
	}

	createdTask, err := c.client.CreateTask(ctx, req)
	if err != nil {
		if c.isTaskAlreadyExistsError(err) {
			return nil, errs.NewResourceAlreadyExistsError(fmt.Errorf("result-check task already exists: %w", err))
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

	queuePath := fmt.Sprintf("projects/%s/locations/%s/queues/%s", c.config.ProjectID, c.config.Region, c.config.NotifySubscriberQueueName)

	payload := map[string]uint{"subscription_id": subscriptionID}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req := &taskspb.CreateTaskRequest{
		Parent: queuePath,
		Task: &taskspb.Task{
			Name:             fmt.Sprintf("%s/tasks/subscription-%d", queuePath, subscriptionID),
			DispatchDeadline: durationpb.New(c.dispatchDeadline),
			MessageType: &taskspb.Task_HttpRequest{
				HttpRequest: &taskspb.HttpRequest{
					HttpMethod: taskspb.HttpMethod_POST,
					Url:        targetURL,
					Body:       body,
					Headers: map[string]string{
						"Content-Type": "application/json",
					},
					AuthorizationHeader: &taskspb.HttpRequest_OidcToken{
						OidcToken: &taskspb.OidcToken{
							ServiceAccountEmail: c.config.ServiceAccountEmail,
							Audience:            c.config.TasksBaseURL,
						},
					},
				},
			},
		},
	}

	if _, err := c.client.CreateTask(ctx, req); err != nil {
		if c.isTaskAlreadyExistsError(err) {
			return errs.NewResourceAlreadyExistsError(fmt.Errorf("subscriber-notification task already exists: %w", err))
		}

		return fmt.Errorf("failed to create subscriber-notification task: %w", err)
	}

	return nil
}

func (c *TaskClient) isTaskAlreadyExistsError(err error) bool {
	return strings.Contains(err.Error(), errAlreadyExists)
}
