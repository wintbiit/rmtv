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

	delivery := lark.Delivery{MaxCards: lark.DefaultMaxCardsPerRound}
	if raw := strings.TrimSpace(os.Getenv("RMTV_MAX_CARDS_PER_ROUND")); raw != "" {
		if count, err := strconv.Atoi(raw); err == nil && count > 0 {
			delivery.MaxCards = count
		} else {
			logrus.Warn("RMTV_MAX_CARDS_PER_ROUND must be a positive integer, using default")
		}
	}
	if larkAppId, ok := os.LookupEnv("LARK_APP_ID"); ok {
		delivery.App = lark.NewClient(larkAppId, os.Getenv("LARK_APP_SECRET"))
		logrus.Infof("enabled lark client with app id: %v", larkAppId)
	}

	if larkWebhooks, ok := os.LookupEnv("LARK_WEBHOOKS"); ok {
		delivery.Webhook = lark.NewWebhookClient(strings.Split(larkWebhooks, ","))
		logrus.Info("enabled lark webhook client")
	}
	if delivery.App != nil || delivery.Webhook != nil {
		j = j.With(job.WithConsumer(delivery))
	}

	if err := j.Run(context.Background()); err != nil {
		logrus.Error(errors.Wrap(err, "failed to run job"))
		os.Exit(1)
	}
}
