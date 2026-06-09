package task_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	taskspb "cloud.google.com/go/cloudtasks/apiv2/cloudtaskspb"
	"github.com/andrewshostak/result-service/config"
	"github.com/andrewshostak/result-service/internal/adapters/http/client/task"
	"github.com/andrewshostak/result-service/internal/adapters/http/client/task/mocks"
	"github.com/andrewshostak/result-service/internal/app/models"
	"github.com/brianvoe/gofakeit/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestTaskClient_GetResultCheckTask(t *testing.T) {
	ctx := context.Background()

	cfg := config.GoogleCloud{
		ProjectID:            gofakeit.Word(),
		Region:               gofakeit.Word(),
		CheckResultQueueName: gofakeit.Word(),
	}

	matchID := uint(gofakeit.Uint16())
	attempt := uint(gofakeit.Uint8())

	queuePath := fmt.Sprintf("projects/%s/locations/%s/queues/%s", cfg.ProjectID, cfg.Region, cfg.CheckResultQueueName)
	taskName := fmt.Sprintf("%s/tasks/match-%d-attempt-%d", queuePath, matchID, attempt)

	scheduleTime := gofakeit.Date().UTC().Truncate(time.Second)

	tests := []struct {
		name             string
		cloudTasksClient func(t *testing.T) *mocks.CloudTasksClient
		expectedResult   *models.Task
		expectedErr      error
	}{
		{
			name: "success - it returns a task when the cloud tasks client call succeeds",
			cloudTasksClient: func(t *testing.T) *mocks.CloudTasksClient {
				t.Helper()
				m := mocks.NewCloudTasksClient(t)
				m.On("GetTask", ctx, &taskspb.GetTaskRequest{Name: taskName}).
					Return(&taskspb.Task{
						Name:         taskName,
						ScheduleTime: timestamppb.New(scheduleTime),
					}, nil).
					Once()
				return m
			},
			expectedResult: &models.Task{
				Name:      taskName,
				ExecuteAt: scheduleTime,
			},
		},
		{
			name: "it returns an error when the cloud tasks client call fails",
			cloudTasksClient: func(t *testing.T) *mocks.CloudTasksClient {
				t.Helper()
				m := mocks.NewCloudTasksClient(t)
				m.On("GetTask", ctx, &taskspb.GetTaskRequest{Name: taskName}).
					Return(nil, errors.New("some error")).
					Once()
				return m
			},
			expectedErr: fmt.Errorf("failed to get result-check task: %w", errors.New("some error")),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := task.NewClient(cfg, time.Minute, tt.cloudTasksClient(t))

			result, err := client.GetResultCheckTask(ctx, matchID, attempt)

			if tt.expectedErr != nil {
				assert.EqualError(t, err, tt.expectedErr.Error())
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestTaskClient_ScheduleResultCheck(t *testing.T) {
	ctx := context.Background()

	cfg := config.GoogleCloud{
		ProjectID:            gofakeit.Word(),
		Region:               gofakeit.Word(),
		CheckResultQueueName: gofakeit.Word(),
		TargetURL:            gofakeit.URL(),
		ServiceAccountEmail:  gofakeit.Email(),
	}

	dispatchDeadline := time.Minute

	matchID := uint(gofakeit.Uint16())
	attempt := uint(gofakeit.Uint8())
	scheduleAt := gofakeit.Date().UTC().Truncate(time.Second)

	queuePath := fmt.Sprintf("projects/%s/locations/%s/queues/%s", cfg.ProjectID, cfg.Region, cfg.CheckResultQueueName)
	taskName := fmt.Sprintf("%s/tasks/match-%d-attempt-%d", queuePath, matchID, attempt)
	targetURL := fmt.Sprintf("%s/v1/triggers/result_check", cfg.TargetURL)

	payload, err := json.Marshal(map[string]uint{"match_id": matchID})
	require.NoError(t, err)

	expectedReq := &taskspb.CreateTaskRequest{
		Parent: queuePath,
		Task: &taskspb.Task{
			Name:             taskName,
			ScheduleTime:     timestamppb.New(scheduleAt),
			DispatchDeadline: durationpb.New(dispatchDeadline),
			MessageType: &taskspb.Task_HttpRequest{
				HttpRequest: &taskspb.HttpRequest{
					HttpMethod: taskspb.HttpMethod_POST,
					Url:        targetURL,
					Body:       payload,
					Headers:    map[string]string{"Content-Type": "application/json"},
					AuthorizationHeader: &taskspb.HttpRequest_OidcToken{
						OidcToken: &taskspb.OidcToken{
							ServiceAccountEmail: cfg.ServiceAccountEmail,
							Audience:            cfg.TargetURL,
						},
					},
				},
			},
		},
	}

	tests := []struct {
		name             string
		cloudTasksClient func(t *testing.T) *mocks.CloudTasksClient
		expectedResult   *models.Task
		expectedErr      error
	}{
		{
			name: "success - it returns a task when the cloud tasks client call succeeds",
			cloudTasksClient: func(t *testing.T) *mocks.CloudTasksClient {
				t.Helper()
				m := mocks.NewCloudTasksClient(t)
				m.On("CreateTask", ctx, expectedReq).
					Return(&taskspb.Task{
						Name:         taskName,
						ScheduleTime: timestamppb.New(scheduleAt),
					}, nil).
					Once()
				return m
			},
			expectedResult: &models.Task{
				Name:      taskName,
				ExecuteAt: scheduleAt,
			},
		},
		{
			name: "it returns an error when the cloud tasks client call fails",
			cloudTasksClient: func(t *testing.T) *mocks.CloudTasksClient {
				t.Helper()
				m := mocks.NewCloudTasksClient(t)
				m.On("CreateTask", ctx, expectedReq).
					Return(nil, errors.New("some error")).
					Once()
				return m
			},
			expectedErr: fmt.Errorf("failed to create result-check task: %w", errors.New("some error")),
		},
		{
			name: "it returns a resource already exists error when the task already exists",
			cloudTasksClient: func(t *testing.T) *mocks.CloudTasksClient {
				t.Helper()
				m := mocks.NewCloudTasksClient(t)
				m.On("CreateTask", ctx, expectedReq).
					Return(nil, errors.New("AlreadyExists")).
					Once()
				return m
			},
			expectedErr: models.NewResourceAlreadyExistsError(fmt.Errorf("result-check task already exists: %w", errors.New("AlreadyExists"))),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := task.NewClient(cfg, dispatchDeadline, tt.cloudTasksClient(t))

			result, err := client.ScheduleResultCheck(ctx, matchID, attempt, scheduleAt)

			if tt.expectedErr != nil {
				assert.EqualError(t, err, tt.expectedErr.Error())
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestTaskClient_DeleteResultCheckTask(t *testing.T) {
	ctx := context.Background()
	cfg := config.GoogleCloud{}

	taskName := gofakeit.Word()

	tests := []struct {
		name             string
		cloudTasksClient func(t *testing.T) *mocks.CloudTasksClient
		expectedErr      error
	}{
		{
			name: "success - it returns no error when the cloud tasks client call succeeds",
			cloudTasksClient: func(t *testing.T) *mocks.CloudTasksClient {
				t.Helper()
				m := mocks.NewCloudTasksClient(t)
				m.On("DeleteTask", ctx, &taskspb.DeleteTaskRequest{Name: taskName}).
					Return(nil).
					Once()
				return m
			},
		},
		{
			name: "it returns an error when the cloud tasks client call fails",
			cloudTasksClient: func(t *testing.T) *mocks.CloudTasksClient {
				t.Helper()
				m := mocks.NewCloudTasksClient(t)
				m.On("DeleteTask", ctx, &taskspb.DeleteTaskRequest{Name: taskName}).
					Return(errors.New("some error")).
					Once()
				return m
			},
			expectedErr: fmt.Errorf("failed to delete result-check task: %w", errors.New("some error")),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := task.NewClient(cfg, time.Minute, tt.cloudTasksClient(t))

			err := client.DeleteResultCheckTask(ctx, taskName)

			if tt.expectedErr != nil {
				assert.EqualError(t, err, tt.expectedErr.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTaskClient_ScheduleSubscriberNotification(t *testing.T) {
	ctx := context.Background()

	cfg := config.GoogleCloud{
		ProjectID:                 gofakeit.Word(),
		Region:                    gofakeit.Word(),
		NotifySubscriberQueueName: gofakeit.Word(),
		TargetURL:                 gofakeit.URL(),
		ServiceAccountEmail:       gofakeit.Email(),
	}

	dispatchDeadline := time.Minute

	subscriptionID := uint(gofakeit.Uint16())

	queuePath := fmt.Sprintf("projects/%s/locations/%s/queues/%s", cfg.ProjectID, cfg.Region, cfg.NotifySubscriberQueueName)
	taskName := fmt.Sprintf("%s/tasks/subscription-%d", queuePath, subscriptionID)
	targetURL := fmt.Sprintf("%s/v1/triggers/subscriber_notification", cfg.TargetURL)

	payload, err := json.Marshal(map[string]uint{"subscription_id": subscriptionID})
	require.NoError(t, err)

	expectedReq := &taskspb.CreateTaskRequest{
		Parent: queuePath,
		Task: &taskspb.Task{
			Name:             taskName,
			DispatchDeadline: durationpb.New(dispatchDeadline),
			MessageType: &taskspb.Task_HttpRequest{
				HttpRequest: &taskspb.HttpRequest{
					HttpMethod: taskspb.HttpMethod_POST,
					Url:        targetURL,
					Body:       payload,
					Headers:    map[string]string{"Content-Type": "application/json"},
					AuthorizationHeader: &taskspb.HttpRequest_OidcToken{
						OidcToken: &taskspb.OidcToken{
							ServiceAccountEmail: cfg.ServiceAccountEmail,
							Audience:            cfg.TargetURL,
						},
					},
				},
			},
		},
	}

	tests := []struct {
		name             string
		cloudTasksClient func(t *testing.T) *mocks.CloudTasksClient
		expectedErr      error
	}{
		{
			name: "success - it returns no error when the cloud tasks client call succeeds",
			cloudTasksClient: func(t *testing.T) *mocks.CloudTasksClient {
				t.Helper()
				m := mocks.NewCloudTasksClient(t)
				m.On("CreateTask", ctx, expectedReq).
					Return(&taskspb.Task{}, nil).
					Once()
				return m
			},
		},
		{
			name: "it returns an error when the cloud tasks client call fails",
			cloudTasksClient: func(t *testing.T) *mocks.CloudTasksClient {
				t.Helper()
				m := mocks.NewCloudTasksClient(t)
				m.On("CreateTask", ctx, expectedReq).
					Return(nil, errors.New("some error")).
					Once()
				return m
			},
			expectedErr: fmt.Errorf("failed to create subscriber-notification task: %w", errors.New("some error")),
		},
		{
			name: "it returns a resource already exists error when the task already exists",
			cloudTasksClient: func(t *testing.T) *mocks.CloudTasksClient {
				t.Helper()
				m := mocks.NewCloudTasksClient(t)
				m.On("CreateTask", ctx, expectedReq).
					Return(nil, errors.New("AlreadyExists")).
					Once()
				return m
			},
			expectedErr: models.NewResourceAlreadyExistsError(fmt.Errorf("subscriber-notification task already exists: %w", errors.New("AlreadyExists"))),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := task.NewClient(cfg, dispatchDeadline, tt.cloudTasksClient(t))

			err := client.ScheduleSubscriberNotification(ctx, subscriptionID)

			if tt.expectedErr != nil {
				assert.EqualError(t, err, tt.expectedErr.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
