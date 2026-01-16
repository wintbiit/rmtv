package lark

import (
	"context"
	"encoding/json"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/wintbiit/rmtv/internal/job"
)

type Config struct {
	AppId           string
	AppSecret       string
	WebhookFilePath string
}

type Client struct {
	client          *lark.Client
	webhookProvider WebhookProvider
}

func NewClient(appId, appSecret string) *Client {
	larkClient := lark.NewClient(appId, appSecret)

	client := &Client{
		client: larkClient,
	}

	if imageUploadClient == nil {
		imageUploadClient = client
	}

	return client
}

func (c *Client) PushMessageToChat(ctx context.Context, chatId string, content string) error {
	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(larkim.ReceiveIdTypeChatId).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(chatId).
			MsgType(larkim.MsgTypeInteractive).
			Content(content).
			Build()).
		Build()

	resp, err := c.client.Im.V1.Message.Create(ctx, req)
	if err != nil {
		return errors.Wrap(err, "failed to create message")
	}

	if !resp.Success() {
		return errors.Wrap(resp, "failed to create message")
	}

	logrus.Infof("successfully pushed message to chat: %s", chatId)
	return nil
}

func (c *Client) PushMessage(ctx context.Context, videos []job.Post) error {
	message, err := BuildMessageCard(ctx, videos)
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

		resp, err := c.client.Im.V1.Message.Create(ctx, req)
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

	return nil
}
