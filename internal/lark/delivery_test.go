package lark

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
)

func TestAppCardsReplyInEachGroupThread(t *testing.T) {
	var mu sync.Mutex
	var roots int
	var groupLimitHits int
	bothRoots := make(chan struct{})
	delivered := make(map[string][]string)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/open-apis/auth/v3/tenant_access_token/internal":
			fmt.Fprint(w, `{"code":0,"tenant_access_token":"fake","expire":7200}`)
		case "/open-apis/im/v1/chats":
			fmt.Fprint(w, `{"code":0,"data":{"items":[{"chat_id":"oc_a"},{"chat_id":"oc_b"}],"has_more":false}}`)
		default:
			var body struct {
				ReceiveID string `json:"receive_id"`
				MsgType   string `json:"msg_type"`
				Content   string `json:"content"`
				Thread    bool   `json:"reply_in_thread"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode request: %v", err)
				http.Error(w, "invalid request", http.StatusBadRequest)
				return
			}
			if body.MsgType != "interactive" {
				t.Errorf("message type = %q", body.MsgType)
			}
			if r.URL.Path == "/open-apis/im/v1/messages" {
				mu.Lock()
				root := "om_" + body.ReceiveID
				delivered[root] = append(delivered[root], body.Content)
				roots++
				if roots == 2 {
					close(bothRoots)
				}
				mu.Unlock()
				select {
				case <-bothRoots:
				case <-r.Context().Done():
					return
				}
				fmt.Fprintf(w, `{"code":0,"data":{"message_id":%q}}`, root)
				return
			}
			var root string
			switch r.URL.Path {
			case "/open-apis/im/v1/messages/om_oc_a/reply":
				root = "om_oc_a"
			case "/open-apis/im/v1/messages/om_oc_b/reply":
				root = "om_oc_b"
			default:
				t.Errorf("unexpected endpoint %q", r.URL.Path)
				http.NotFound(w, r)
				return
			}
			if !body.Thread {
				t.Error("reply_in_thread must be true")
			}
			mu.Lock()
			if root == "om_oc_a" && body.Content == "second" && groupLimitHits == 0 {
				groupLimitHits++
				mu.Unlock()
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprint(w, `{"code":230020,"msg":"group rate limit"}`)
				return
			}
			delivered[root] = append(delivered[root], body.Content)
			mu.Unlock()
			fmt.Fprint(w, `{"code":0,"data":{"message_id":"om_reply"}}`)
		}
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client := &Client{client: lark.NewClient("thread-test", "secret", lark.WithOpenBaseUrl(server.URL))}
	cards := []preparedCard{{content: "first", identity: "1"}, {content: "second", identity: "2"}, {content: "third", identity: "3"}}
	if err := client.pushCards(ctx, cards); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if groupLimitHits != 1 {
		t.Fatalf("expected to exercise the shared group limit, got %d hits", groupLimitHits)
	}
	want := []string{"first", "second", "third"}
	for _, root := range []string{"om_oc_a", "om_oc_b"} {
		if !reflect.DeepEqual(delivered[root], want) {
			t.Errorf("%s got %v, want %v", root, delivered[root], want)
		}
	}
}
