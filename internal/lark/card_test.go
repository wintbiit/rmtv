package lark

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/wintbiit/rmtv/internal/job"
)

func TestMixedCardsPreserveEveryEntry(t *testing.T) {
	entries := make([]cardEntry, 24)
	for i := range entries {
		source := []string{"Bilibili", "轻流", "RMBBS"}[i%3]
		entries[i].Item = job.Item{Source: source, ID: fmt.Sprint(i), Title: fmt.Sprintf("标题 %d", i),
			URL: fmt.Sprintf("https://example.com/entry/%02d", i), Body: strings.Repeat("正文", 270),
			Author: "作者甲", PublishedAt: time.Date(2026, 11, 16, 7, 15, 0, 0, time.UTC)}
	}
	entries[0].pictures = []cardImage{{Key: "img_cover"}, {Key: "img_next"}}
	entries[1].Number = "481"
	entries[1].pictures = []cardImage{{Key: "img_diagram", Caption: "问题附图", AfterTitle: true}}
	cards, err := packCards(entries)
	if err != nil || len(cards) < 2 {
		t.Fatalf("expected multiple cards, got %d: %v", len(cards), err)
	}
	var combined strings.Builder
	for _, page := range cards {
		if !fitsCard(page) {
			t.Fatal("oversized page")
		}
		body, err := json.Marshal(page)
		if err != nil {
			t.Fatal(err)
		}
		combined.Write(body)
	}
	payload := combined.String()
	for i := range entries {
		if count := strings.Count(payload, fmt.Sprintf("https://example.com/entry/%02d", i)); count != 1 {
			t.Errorf("entry %d appeared %d times", i, count)
		}
	}
	for _, expected := range []string{"img_combination", "img_diagram", "collapsible_panel", "作者甲 · 11-16 15:15", "#481 · 11-16 15:15", "summary"} {
		if !strings.Contains(payload, expected) {
			t.Errorf("missing %q", expected)
		}
	}
}

func TestQingFlowDetailOrderAndInlineChip(t *testing.T) {
	entry := cardEntry{Item: job.Item{Source: "轻流", Title: "完整问题", Body: "完整答复", URL: "https://qingflow.com"},
		pictures: []cardImage{
			{Key: "question_key", Caption: "问题附图", AfterTitle: true},
			{Key: "answer_key", Caption: "答复附图"},
		}}
	body, err := json.Marshal(renderEntry(entry, true))
	if err != nil {
		t.Fatal(err)
	}
	payload := string(body)
	if !strings.Contains(payload, `轻流\u003c/text_tag\u003e  [完整问题]`) {
		t.Errorf("source chip and title must share an inline element: %s", payload)
	}
	want := []string{"**问题**", "问题附图", "**答复**", "答复附图"}
	position := 0
	for _, segment := range want {
		index := strings.Index(payload[position:], segment)
		if index < 0 {
			t.Fatalf("missing or out of order: %q", segment)
		}
		position += index + len(segment)
	}
	if !strings.Contains(payload, "查看全文") || strings.Contains(payload, "张图") {
		t.Fatal("incorrect panel label")
	}
}

func TestOversizedDetailsLeaveTheOriginalLink(t *testing.T) {
	entry := cardEntry{Item: job.Item{Title: "原始问题", URL: "https://example.com/a", Body: strings.Repeat("正文", 20000)}}
	cards, err := packCards([]cardEntry{entry})
	if err != nil || len(cards) != 1 {
		t.Fatalf("got %d cards: %v", len(cards), err)
	}
	body, _ := json.Marshal(cards[0])
	if strings.Contains(string(body), "collapsible_panel") || !strings.Contains(string(body), entry.URL) {
		t.Fatal("oversized details should link to the original")
	}
}

func TestWebhookRejectsFeishuErrorWithHTTP200(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var payload ChatContent
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.Card.Schema != "2.0" || payload.MsgType != "interactive" {
			t.Errorf("invalid webhook payload: %v", err)
		}
		if calls == 1 {
			fmt.Fprint(w, `{"code":11232}`)
			return
		}
		fmt.Fprint(w, `{"StatusCode":0}`)
	}))
	defer server.Close()
	client := NewWebhookClient([]string{server.URL})
	card := renderPage([]cardEntry{{Item: job.Item{Title: "example"}}}, nil, "")
	if err := retryCard(context.Background(), func() (bool, error) { return client.sendCard(context.Background(), server.URL, card) }); err != nil || calls != 2 {
		t.Fatalf("calls=%d error=%v", calls, err)
	}
}
