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
	AppId     string
	AppSecret string
}

type Client struct {
	webhooks   []string
	client     *resty.Client
	larkClient *lark.Client
}

func NewClient(webhooks []string, config *Config) *Client {
	if len(webhooks) == 0 {
		return nil
	}

	c := resty.New().
		SetRetryCount(3).
		SetRetryWaitTime(2 * time.Second).
		SetRetryMaxWaitTime(10 * time.Second).
		SetDebug(utils.Debug).
		SetTimeout(10 * time.Second)

	larkClient := lark.NewClient(config.AppId, config.AppSecret, lark.WithHttpClient(c.Client()))

	client := &Client{
		client:     c,
		webhooks:   webhooks,
		larkClient: larkClient,
	}

	return client
}

func (c *Client) PushMessage(ctx context.Context, videos []bilibili.SearchResult) error {
	if len(c.webhooks) == 0 {
		return nil
	}

	message, err := c.buildMessageCard(ctx, videos)
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
			return err
		}

		if !resp.IsSuccess() {
			return errors.Wrapf(err, "lark push message failed: %s", resp.String())
		}
	}

	messageData, _ := json.Marshal(message)
	return c.ForeachChat(ctx, func(chat *larkim.ListChat) {
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
	})
}
