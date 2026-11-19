package task

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	taskspb "cloud.google.com/go/cloudtasks/apiv2/cloudtaskspb"
	"github.com/andrewshostak/result-service/config"
	"github.com/andrewshostak/result-service/internal/app/models"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	providerResultCheckPath = "/v1/triggers/provider_result_check"
	notifySubscriberPath    = "/v1/triggers/subscriber_notification"
)

const (
	errAlreadyExists = "AlreadyExists"
)

type TaskClient struct {
	client           CloudTasksClient
	config           config.GoogleCloud
	dispatchDeadline time.Duration
}

func NewClient(config config.GoogleCloud, dispatchDeadline time.Duration, client CloudTasksClient) *TaskClient {
	return &TaskClient{config: config, dispatchDeadline: dispatchDeadline, client: client}
}

func (c *TaskClient) EnsureProviderResultCheckScheduled(ctx context.Context, providerMatchID uint, attempt uint, scheduleAt time.Time) (*models.Task, error) {
	targetURL := fmt.Sprintf("%s%s", c.config.TargetURL, providerResultCheckPath)

	queuePath := fmt.Sprintf("projects/%s/locations/%s/queues/%s", c.config.ProjectID, c.config.Region, c.config.CheckResultQueueName)

	payload := map[string]uint{"provider_match_id": providerMatchID}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req := &taskspb.CreateTaskRequest{
		Parent: queuePath,
		Task: &taskspb.Task{
			Name:             fmt.Sprintf("%s/tasks/provider-match-%d-attempt-%d", queuePath, providerMatchID, attempt),
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
							Audience:            c.config.TargetURL,
						},
					},
				},
			},
		},
	}

	createdTask, err := c.client.CreateTask(ctx, req)
	if err != nil {
		if c.isTaskAlreadyExistsError(err) {
			existingTask, errGet := c.client.GetTask(ctx, &taskspb.GetTaskRequest{Name: req.Task.Name})
			if errGet != nil {
				return nil, fmt.Errorf("failed to get existing provider result-check task: %w", errGet)
			}

			return &models.Task{Name: existingTask.Name, ExecuteAt: existingTask.ScheduleTime.AsTime()}, nil
		}

		return nil, fmt.Errorf("failed to create provider result-check task: %w", err)
	}

	return &models.Task{
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
	targetURL := fmt.Sprintf("%s%s", c.config.TargetURL, notifySubscriberPath)

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
							Audience:            c.config.TargetURL,
						},
					},
				},
			},
		},
	}

	if _, err := c.client.CreateTask(ctx, req); err != nil {
		if c.isTaskAlreadyExistsError(err) {
			return models.NewResourceAlreadyExistsError(fmt.Errorf("subscriber-notification task already exists: %w", err))
		}

		return fmt.Errorf("failed to create subscriber-notification task: %w", err)
	}

	return nil
}

func (c *TaskClient) isTaskAlreadyExistsError(err error) bool {
	return strings.Contains(err.Error(), errAlreadyExists)
}
