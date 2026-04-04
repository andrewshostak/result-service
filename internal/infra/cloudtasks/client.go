package cloudtasks

import (
	"context"

	cloudtasks "cloud.google.com/go/cloudtasks/apiv2"
	"github.com/andrewshostak/result-service/config"
	"github.com/gin-gonic/gin"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func NewClient(ctx context.Context, mode string, cfg config.GoogleCloud) (*cloudtasks.Client, error) {
	var opts []option.ClientOption

	if mode == gin.TestMode {
		opts = append(opts,
			option.WithEndpoint(cfg.TasksURL),
			option.WithoutAuthentication(),
			option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
		)
	}

	return cloudtasks.NewClient(ctx, opts...)
}
