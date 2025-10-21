package qflow

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/sirupsen/logrus"
	"scutbot.cn/web/rmtv/internal/lark"
)

func TestCollectQFlow(t *testing.T) {
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
	webhookFilePath := "webhooks.txt"
	if wbpOverride, ok := os.LookupEnv("LARK_WEBHOOK_FILE_PATH"); ok {
		webhookFilePath = wbpOverride
	}

	larkClient := lark.NewClient(&lark.Config{
		AppId:           larkClientId,
		AppSecret:       larkClientSecret,
		WebhookFilePath: webhookFilePath,
	})

	card, err := larkClient.BuildMessageCard(t.Context(), entries[:5])
	if err != nil {
		t.Fatal(err)
	}

	spew.Dump(card)

	data, err := json.Marshal(card)
	if err != nil {
		t.Fatal(err)
	}

	t.Log(string(data))

	err = larkClient.PushMessageToChat(t.Context(), "oc_a9043042ede841a61ba05e9effdf60ca", string(data))
	if err != nil {
		t.Fatal(err)
	}

	t.Log("ok")
}
