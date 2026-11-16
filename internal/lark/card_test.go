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
	entries[0].pictures = []cardImage{{"img_cover", ""}, {"img_next", ""}}
	entries[1].Number = "481"
	entries[1].pictures = []cardImage{{"img_diagram", "问题附图"}}
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
