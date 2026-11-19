package lark

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"go.uber.org/ratelimit"
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
	return &WebhookClient{client: resty.New().SetTimeout(10 * time.Second), webhooks: webhooks}
}

func (c *WebhookClient) pushCards(ctx context.Context, cards []preparedCard) error {
	var webhooks []string
	seen := make(map[string]bool)
	for _, address := range c.webhooks {
		address = strings.TrimSpace(address)
		if address != "" && !seen[address] {
			seen[address] = true
			webhooks = append(webhooks, address)
		}
	}
	return sendToDestinations(ctx, len(webhooks), func(i int) error {
		// Feishu custom bots allow 100 requests/minute and five/second.
		pace := ratelimit.New(100, ratelimit.Per(time.Minute), ratelimit.WithoutSlack)
		for page, card := range cards {
			err := retryCard(ctx, func() (bool, error) {
				pace.Take()
				if err := ctx.Err(); err != nil {
					return false, err
				}
				return c.sendCard(ctx, webhooks[i], card.webhook)
			})
			if err != nil {
				return fmt.Errorf("webhook %d card %d/%d: %w", i+1, page+1, len(cards), err)
			}
			logrus.WithFields(logrus.Fields{"webhook_index": i + 1, "page": page + 1, "pages": len(cards)}).Info("Sent RMTV card")
		}
		return nil
	})
}

func (c *WebhookClient) sendCard(ctx context.Context, webhook string, body []byte) (bool, error) {
	response, err := c.client.R().SetContext(ctx).SetHeader("Content-Type", "application/json").
		SetBody(body).Post(webhook)
	if err != nil {
		return false, errors.New("webhook request failed")
	}
	if response.StatusCode() == http.StatusTooManyRequests {
		return true, fmt.Errorf("webhook HTTP %d", response.StatusCode())
	}
	if !response.IsSuccess() {
		return false, fmt.Errorf("webhook HTTP %d", response.StatusCode())
	}
	var result struct {
		Code       *int
		StatusCode *int
	}
	if err := json.Unmarshal(response.Bytes(), &result); err != nil {
		return false, errors.New("invalid webhook acknowledgement")
	}
	code := result.Code
	if code == nil {
		code = result.StatusCode
	}
	if code == nil {
		return false, errors.New("missing webhook acknowledgement")
	}
	if *code != 0 {
		return *code == 11232 || *code == 99991400, fmt.Errorf("webhook response code %d", *code)
	}
	return false, nil
}
