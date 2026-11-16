package lark

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"time"

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
	larkClient := lark.NewClient(appId, appSecret, lark.WithReqTimeout(20*time.Second))

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

func (c *Client) PushMessage(ctx context.Context, posts []job.Post) error {
	cards, err := BuildMessageCards(ctx, posts)
	if err != nil || len(cards) == 0 {
		return err
	}
	var failures []error
	err = c.ForeachChat(ctx, func(chat *larkim.ListChat) {
		if chat.ChatId == nil {
			return
		}
		for i, card := range cards {
			if err := c.sendCard(ctx, *chat.ChatId, card); err != nil {
				failures = append(failures, fmt.Errorf("chat %s card %d/%d: %w", *chat.ChatId, i+1, len(cards), err))
				break
			}
			logrus.WithFields(logrus.Fields{"chat_id": *chat.ChatId, "page": i + 1, "pages": len(cards)}).Info("Sent RMTV card")
		}
	})
	return stderrors.Join(append(failures, err)...)
}

func (c *Client) sendCard(ctx context.Context, chatID string, card *ChatCard) error {
	content, err := json.Marshal(card)
	if err != nil {
		return err
	}
	digest := sha256.Sum256([]byte(chatID + "\x00" + card.identity))
	request := larkim.NewCreateMessageReqBuilder().ReceiveIdType(larkim.ReceiveIdTypeChatId).
		Body(larkim.NewCreateMessageReqBodyBuilder().ReceiveId(chatID).
			MsgType(larkim.MsgTypeInteractive).Content(string(content)).
			Uuid(fmt.Sprintf("%x", digest[:16])).Build()).Build()
	return retryCard(ctx, func() (bool, error) {
		result, err := c.client.Im.V1.Message.Create(ctx, request)
		if err != nil {
			return true, err
		}
		if result.Success() {
			return false, nil
		}
		status := 0
		if result.ApiResp != nil {
			status = result.StatusCode
		}
		return status == 429 || status >= 500 || result.Code == 99991400, fmt.Errorf("Feishu message API code %d: %s", result.Code, result.Msg)
	})
}

// Retry one card so success for another page does not cause it to be resent here.
func retryCard(ctx context.Context, send func() (bool, error)) error {
	for attempt := 0; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		retry, err := send()
		if err == nil || !retry || attempt == 2 {
			return err
		}
		timer := time.NewTimer(time.Second << attempt)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
