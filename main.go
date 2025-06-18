package main

import (
	"context"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"os"
	"scutbot.cn/web/rmtv/internal/job"
	"strings"
)

func main() {
	keywords, ok := os.LookupEnv("KEYWORDS")
	if !ok {
		keywords = "RoboMaster,机甲大师"
		logrus.Warnf("KEYWORDS environment variable not set. Using default keywords: %s", keywords)
	}

	webhooks, ok := os.LookupEnv("LARK_WEBHOOKS")
	if !ok {
		logrus.Warn("LARK_WEBHOOKS environment variable not set. Webhook is disabled.")
	}

	j := job.NewTvJob(
		strings.Split(keywords, ","),
		job.WithLarkWebhooks(strings.Split(webhooks, ",")),
	)

	if err := j.Run(context.Background()); err != nil {
		panic(errors.Wrap(err, "failed to run job"))
	}
}
