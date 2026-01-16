package main

import (
	"context"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/wintbiit/rmtv/internal/bilibili"
	"github.com/wintbiit/rmtv/internal/job"
	"github.com/wintbiit/rmtv/internal/lark"
	"github.com/wintbiit/rmtv/internal/qflow"
	"github.com/wintbiit/rmtv/internal/rmbbs"
)

var modules = map[string]func() job.MessageProvider{
	bilibili.Module: func() job.MessageProvider {
		return bilibili.NewClient()
	},
	rmbbs.Module: func() job.MessageProvider {
		return rmbbs.NewClient()
	},
	qflow.Module: func() job.MessageProvider {
		return qflow.NewClient()
	},
}

func main() {
	godotenv.Load()

	db, ok := os.LookupEnv("DB_URL")
	if !ok {
		panic("DB_URL is required")
	}

	enableModules, ok := os.LookupEnv("ENABLE_MODULES")
	if !ok {
		enableModules = "bilibili,rmbbs,qflow"
	}
	logrus.Infof("enabled modules: %v", enableModules)

	j := job.NewTvJob(
		job.WithDb(db),
	)

	for _, module := range strings.Split(enableModules, ",") {
		if f, ok := modules[module]; ok {
			j = j.With(job.WithProvider(f()))
		}
	}

	if larkAppId, ok := os.LookupEnv("LARK_APP_ID"); ok {
		j = j.With(job.WithConsumer(lark.NewClient(larkAppId, os.Getenv("LARK_APP_SECRET"))))
		logrus.Infof("enabled lark client with app id: %v", larkAppId)
	}

	if larkWebhooks, ok := os.LookupEnv("LARK_WEBHOOKS"); ok {
		j = j.With(job.WithConsumer(lark.NewWebhookClient(strings.Split(larkWebhooks, ","))))
		logrus.Infof("enabled lark webhook client with file: %v", larkWebhooks)
	}

	if maxCountPerPush, ok := os.LookupEnv("MAX_COUNT_PER_PUSH"); ok {
		if count, err := strconv.Atoi(maxCountPerPush); err == nil && count > 0 {
			j = j.With(job.WithMaxCountPerPush(count))
		}
	}

	if err := j.Run(context.Background()); err != nil {
		logrus.Error(errors.Wrap(err, "failed to run job"))
		os.Exit(1)
	}
}
