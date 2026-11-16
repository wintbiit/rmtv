package lark

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/wintbiit/rmtv/internal/job"
)

const DefaultMaxCardsPerRound = 2

// Delivery prepares one batch per scan and reuses it across destinations.
type Delivery struct {
	App      *Client
	Webhook  *WebhookClient
	MaxCards int
}

type preparedCard struct {
	content  string
	webhook  []byte
	identity string
}

func (d Delivery) PushMessage(ctx context.Context, posts []job.Post) error {
	limit := d.MaxCards
	if limit <= 0 {
		limit = DefaultMaxCardsPerRound
	}
	cards, err := buildMessageCards(ctx, posts, limit)
	if err != nil || len(cards) == 0 {
		return err
	}
	prepared := make([]preparedCard, 0, len(cards))
	for _, card := range cards {
		content, err := json.Marshal(card)
		if err != nil {
			return fmt.Errorf("encode Feishu card: %w", err)
		}
		page := preparedCard{content: string(content), identity: card.identity}
		if d.Webhook != nil {
			page.webhook, err = json.Marshal(ChatContent{MsgType: "interactive", Card: card})
			if err != nil {
				return fmt.Errorf("encode Feishu webhook card: %w", err)
			}
		}
		prepared = append(prepared, page)
	}
	var appErr, webhookErr error
	if d.App != nil {
		appErr = d.App.pushCards(ctx, prepared)
	}
	if d.Webhook != nil {
		webhookErr = d.Webhook.pushCards(ctx, prepared)
	}
	return errors.Join(appErr, webhookErr)
}
