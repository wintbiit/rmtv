package lark

import (
	"context"
	"time"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/wintbiit/rmtv/internal/job"
	"github.com/wintbiit/rmtv/utils"
	"resty.dev/v3"
)

type WebhookProvider interface {
	GetWebhooks() ([]string, error)
}

type WebhookClient struct {
	client   *resty.Client
	webhooks []string
}

func NewWebhookClient(webhooks []string) *WebhookClient {
	c := resty.New().
		SetRetryCount(3).
		SetRetryWaitTime(2 * time.Second).
		SetRetryMaxWaitTime(10 * time.Second).
		SetDebug(utils.Debug).
		SetTimeout(10 * time.Second)

	client := &WebhookClient{
		client:   c,
		webhooks: webhooks,
	}

	return client
}

func (c *WebhookClient) PushMessage(ctx context.Context, videos []job.Post) error {
	message, err := BuildMessageCard(ctx, videos)
	if err != nil {
		return err
	}

	for _, webhook := range c.webhooks {
		resp, err := c.client.R().
			SetContext(ctx).
			SetHeader("Content-Type", "application/json").
			SetBody(ChatContent{
				MsgType: larkim.MsgTypeInteractive,
				Card:    message,
			}).
			Post(webhook)
		if err != nil {
			logrus.Errorf("failed to post webhook: %v", err)
			continue
		}

		if !resp.IsSuccess() {
			logrus.Error(errors.Wrapf(err, "lark push message failed: %s", resp.String()))
		}

		logrus.Infof("successfully pushed message to webhook: %s", webhook)
	}

	return nil
}
