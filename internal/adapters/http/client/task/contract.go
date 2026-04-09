package task

import (
	"context"

	taskspb "cloud.google.com/go/cloudtasks/apiv2/cloudtaskspb"
	"github.com/googleapis/gax-go/v2"
)

type CloudTasksClient interface {
	GetTask(ctx context.Context, task *taskspb.GetTaskRequest, opts ...gax.CallOption) (*taskspb.Task, error)
	CreateTask(ctx context.Context, task *taskspb.CreateTaskRequest, opts ...gax.CallOption) (*taskspb.Task, error)
	DeleteTask(ctx context.Context, task *taskspb.DeleteTaskRequest, opts ...gax.CallOption) error
}
