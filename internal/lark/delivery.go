package lark

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"sync"

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
	items := cardItems(posts)
	entries := cardEntries(ctx, items)
	var appErr, webhookErr error
	if d.App != nil {
		pages, err := prepareCards(ctx, entries, len(items), 0, false)
		if err == nil && len(pages) > 0 {
			err = d.App.pushCards(ctx, pages)
		}
		appErr = err
	}
	if d.Webhook != nil {
		pages, err := prepareCards(ctx, entries, len(items), limit, true)
		if err == nil && len(pages) > 0 {
			err = d.Webhook.pushCards(ctx, pages)
		}
		webhookErr = err
	}
	return errors.Join(appErr, webhookErr)
}

func prepareCards(ctx context.Context, entries iter.Seq[cardEntry], total, limit int, webhook bool) ([]preparedCard, error) {
	cards, err := packCards(entries, total, limit)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		return nil, err
	}
	prepared := make([]preparedCard, 0, len(cards))
	for _, card := range cards {
		page := preparedCard{identity: card.identity}
		if webhook {
			page.webhook, err = json.Marshal(ChatContent{MsgType: "interactive", Card: card})
		} else {
			var content []byte
			content, err = json.Marshal(card)
			page.content = string(content)
		}
		if err != nil {
			return nil, fmt.Errorf("encode Feishu card: %w", err)
		}
		prepared = append(prepared, page)
	}
	return prepared, nil
}

func sendToDestinations(ctx context.Context, count int, send func(int) error) error {
	failures := make([]error, count)
	var pending sync.WaitGroup
	for i := range count {
		if ctx.Err() != nil {
			break
		}
		pending.Add(1)
		go func() {
			defer pending.Done()
			failures[i] = send(i)
		}()
	}
	pending.Wait()
	return errors.Join(append(failures, ctx.Err())...)
}
