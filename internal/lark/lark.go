package lark

import (
	"context"
	"crypto/sha256"
	stderrors "errors"
	"fmt"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"go.uber.org/ratelimit"
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

func (c *Client) pushCards(ctx context.Context, cards []preparedCard) error {
	var chatIDs []string
	seen := make(map[string]bool)
	listErr := c.ForeachChat(ctx, func(chat *larkim.ListChat) {
		if chat.ChatId != nil && *chat.ChatId != "" && !seen[*chat.ChatId] {
			seen[*chat.ChatId] = true
			chatIDs = append(chatIDs, *chat.ChatId)
		}
	})
	// Use one conservative request budget for creates and replies across all groups.
	appPace := ratelimit.New(1000, ratelimit.Per(time.Minute), ratelimit.WithoutSlack)
	sendErr := sendToDestinations(ctx, len(chatIDs), func(i int) error {
		chatID := chatIDs[i]
		// Feishu shares 5 QPS among all bots in a group. This only paces our calls.
		groupPace := ratelimit.New(5, ratelimit.WithoutSlack)
		var rootID string
		for page, card := range cards {
			err := retryCard(ctx, func() (bool, error) {
				groupPace.Take()
				appPace.Take()
				if err := ctx.Err(); err != nil {
					return false, err
				}
				if page == 0 {
					var err error
					var retry bool
					rootID, retry, err = c.sendRootCard(ctx, chatID, card, len(cards) > 1)
					return retry, err
				}
				return c.replyCard(ctx, rootID, card)
			})
			if err != nil {
				return fmt.Errorf("chat %s card %d/%d: %w", chatID, page+1, len(cards), err)
			}
			logrus.WithFields(logrus.Fields{"chat_id": chatID, "page": page + 1, "pages": len(cards)}).Info("Sent RMTV card")
		}
		return nil
	})
	return stderrors.Join(listErr, sendErr)
}

func cardUUID(destination string, card preparedCard) string {
	digest := sha256.Sum256([]byte(destination + "\x00" + card.identity))
	return fmt.Sprintf("%x", digest[:16])
}

func (c *Client) sendRootCard(ctx context.Context, chatID string, card preparedCard, expectReplies bool) (string, bool, error) {
	request := larkim.NewCreateMessageReqBuilder().ReceiveIdType(larkim.ReceiveIdTypeChatId).
		Body(larkim.NewCreateMessageReqBodyBuilder().ReceiveId(chatID).
			MsgType(larkim.MsgTypeInteractive).Content(card.content).
			Uuid(cardUUID(chatID, card)).Build()).Build()
	result, err := c.client.Im.V1.Message.Create(ctx, request)
	if err != nil {
		return "", true, err
	}
	if !result.Success() {
		retry, err := cardSendError(result.ApiResp, result.CodeError)
		return "", retry, err
	}
	if !expectReplies {
		return "", false, nil
	}
	if result.Data == nil || result.Data.MessageId == nil || *result.Data.MessageId == "" {
		return "", false, fmt.Errorf("Feishu did not return a message ID for thread replies")
	}
	return *result.Data.MessageId, false, nil
}

func (c *Client) replyCard(ctx context.Context, rootID string, card preparedCard) (bool, error) {
	request := larkim.NewReplyMessageReqBuilder().MessageId(rootID).
		Body(larkim.NewReplyMessageReqBodyBuilder().MsgType(larkim.MsgTypeInteractive).
			Content(card.content).ReplyInThread(true).Uuid(cardUUID(rootID, card)).Build()).Build()
	result, err := c.client.Im.V1.Message.Reply(ctx, request)
	if err != nil {
		return true, err
	}
	if result.Success() {
		return false, nil
	}
	return cardSendError(result.ApiResp, result.CodeError)
}

func cardSendError(resp *larkcore.ApiResp, code larkcore.CodeError) (bool, error) {
	status := 0
	if resp != nil {
		status = resp.StatusCode
	}
	retry := status == 429 || status >= 500 || code.Code == 99991400 || code.Code == 230020
	return retry, fmt.Errorf("Feishu message API code %d: %s", code.Code, code.Msg)
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
