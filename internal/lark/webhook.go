package lark

import (
	"context"
	"os"
	"strings"
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

type fileWebhookProvider struct {
	filePath string
}

func (f fileWebhookProvider) GetWebhooks() ([]string, error) {
	content, err := os.ReadFile(f.filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			logrus.Warnf("webhooks file at %s not set.", f.filePath)
			return nil, nil
		} else {
			return nil, errors.Wrapf(err, "failed to read file %s", f.filePath)
		}
	}

	return strings.Split(string(content), "\n"), nil
}

func NewFileWebhookProvider(filePath string) WebhookProvider {
	return &fileWebhookProvider{filePath: filePath}
}

type WebhookClient struct {
	client          *resty.Client
	webhookProvider WebhookProvider
}

func NewWebhookClient(filepath string) *WebhookClient {
	c := resty.New().
		SetRetryCount(3).
		SetRetryWaitTime(2 * time.Second).
		SetRetryMaxWaitTime(10 * time.Second).
		SetDebug(utils.Debug).
		SetTimeout(10 * time.Second)

	client := &WebhookClient{
		client:          c,
		webhookProvider: NewFileWebhookProvider(filepath),
	}

	return client
}

func (c *WebhookClient) PushMessage(ctx context.Context, videos []job.MessageEntry) error {
	message, err := BuildMessageCard(ctx, videos)
	if err != nil {
		return err
	}

	webhooks, err := c.webhookProvider.GetWebhooks()
	if err != nil {
		return errors.Wrap(err, "failed to get webhooks")
	}

	for _, webhook := range webhooks {
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
