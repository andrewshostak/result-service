package client

import (
	"context"
	"time"

	cloudtasks "cloud.google.com/go/cloudtasks/apiv2"
	"github.com/andrewshostak/result-service/config"
)

const (
	checkResultQueue      = "check-result"
	notifySubscriberQueue = "notify-subscriber"
)

type TaskClient struct {
	client *cloudtasks.Client
	config config.GoogleCloud
}

func NewClient(config config.GoogleCloud, client *cloudtasks.Client) *TaskClient {
	return &TaskClient{config: config, client: client}
}

func (c *TaskClient) ScheduleResultCheck(ctx context.Context, matchID uint, scheduleAt time.Time) error {
	// TODO
	return nil
}

func (c *TaskClient) ScheduleSubscriberNotification(ctx context.Context, subscriptionID uint) error {
	panic("implement me")
}
