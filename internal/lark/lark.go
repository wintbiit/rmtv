package lark

import (
	"context"
	"encoding/json"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"resty.dev/v3"
	"scutbot.cn/web/rmtv/internal/bilibili"
	"scutbot.cn/web/rmtv/utils"
)

type Config struct {
	AppId           string
	AppSecret       string
	WebhookFilePath string
}

type Client struct {
	client          *resty.Client
	larkClient      *lark.Client
	webhookProvider WebhookProvider
}

func NewClient(config *Config) *Client {
	c := resty.New().
		SetRetryCount(3).
		SetRetryWaitTime(2 * time.Second).
		SetRetryMaxWaitTime(10 * time.Second).
		SetDebug(utils.Debug).
		SetTimeout(10 * time.Second)

	larkClient := lark.NewClient(config.AppId, config.AppSecret, lark.WithHttpClient(c.Client()))

	var provider WebhookProvider
	if config.WebhookFilePath != "" {
		provider = NewFileWebhookProvider(config.WebhookFilePath)
	}

	client := &Client{
		client:          c,
		larkClient:      larkClient,
		webhookProvider: provider,
	}

	return client
}

func (c *Client) PushMessage(ctx context.Context, videos []bilibili.SearchResult) error {
	message, err := c.buildMessageCard(ctx, videos)
	if err != nil {
		return err
	}

	messageData, _ := json.Marshal(message)
	if err = c.ForeachChat(ctx, func(chat *larkim.ListChat) {
		req := larkim.NewCreateMessageReqBuilder().
			ReceiveIdType(larkim.ReceiveIdTypeChatId).
			Body(larkim.NewCreateMessageReqBodyBuilder().
				ReceiveId(*chat.ChatId).
				MsgType(larkim.MsgTypeInteractive).
				Content(string(messageData)).
				Build()).
			Build()

		resp, err := c.larkClient.Im.V1.Message.Create(ctx, req)
		if err != nil {
			logrus.Errorf("failed to create message: %v", err)
			return
		}

		if !resp.Success() {
			logrus.Error(errors.Wrap(resp, "failed to create message"))
			return
		}

		logrus.Infof("successfully pushed message to chat: %s(%s)", *chat.Name, *chat.ChatId)
	}); err != nil {
		logrus.Errorf("failed to push message to chat: %v", err)
	}

	if c.webhookProvider != nil {
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
	}

	return nil
}
