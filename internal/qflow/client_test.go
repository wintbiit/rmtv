package qflow

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"resty.dev/v3"
)

type qflowTestTransport func(*http.Request) (*http.Response, error)

func (f qflowTestTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestCollectMixedTimestampFields(t *testing.T) {
	client := &Client{client: resty.New().SetTransport(qflowTestTransport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{
			"data": {"list": [
				{"applyId":1,"applyNum":480,"answers":[
					{"queTitle":"申请时间","values":[{"value":"2026-11-12 19:51:05"}]},
					{"queTitle":"更新时间","values":[{"value":"2026-11-12 19:51:05"}]}
				]},
				{"applyId":2,"applyNum":479,"applyBaseInfo":{"applyTime":"invalid"}},
				{"applyId":802771327,"applyNum":481,"createTime":null,"lastUpdateTime":1794484265000,
				 "applyBaseInfo":{"applyTime":"2026-11-12T19:51:05","lastUpdateTime":"2026-11-12T19:51:05"}}
			]}
		}`))}, nil
	})), appId: "app", baseId: "view"}

	got, err := client.Collect()
	if err != nil || len(got) != 2 {
		t.Fatalf("collected %d records, want 2, error: %v", len(got), err)
	}
	wantTime := time.Date(2026, 11, 12, 11, 51, 5, 0, time.UTC)
	for i, wantID := range []string{"1", "802771327"} {
		answer := got[i].(*Answer)
		if answer.ID != wantID || !answer.CreatedAt.Equal(wantTime) || !answer.UpdatedAt.Equal(wantTime) {
			t.Errorf("record %d: got %s (%s, %s), want %s (%s)", i, answer.ID, answer.CreatedAt, answer.UpdatedAt, wantID, wantTime)
		}
	}
}
