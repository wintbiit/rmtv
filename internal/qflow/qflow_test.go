package qflow

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/sirupsen/logrus"
	"github.com/wintbiit/rmtv/internal/lark"
)

func TestCollectQFlow(t *testing.T) {
	ctx := context.Background()
	client := NewClient()
	entries, err := client.Collect()
	if err != nil {
		t.Fatal(err)
	}

	spew.Dump(entries)

	larkClientId, ok := os.LookupEnv("LARK_APP_ID")
	if !ok {
		logrus.Fatal("LARK_APP_ID is not set")
	}
	larkClientSecret, ok := os.LookupEnv("LARK_APP_SECRET")
	if !ok {
		logrus.Fatal("LARK_APP_SECRET is not set")
	}

	larkClient := lark.NewClient(larkClientId, larkClientSecret)

	card, err := lark.BuildMessageCard(ctx, entries[:5])
	if err != nil {
		t.Fatal(err)
	}

	spew.Dump(card)

	data, err := json.Marshal(card)
	if err != nil {
		t.Fatal(err)
	}

	t.Log(string(data))

	err = larkClient.PushMessageToChat(ctx, "oc_a9043042ede841a61ba05e9effdf60ca", string(data))
	if err != nil {
		t.Fatal(err)
	}

	t.Log("ok")
}
