package main

import (
	"context"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/wintbiit/rmtv/internal/lark"
	"github.com/wintbiit/rmtv/internal/qflow"

	"github.com/wintbiit/rmtv/internal/bilibili"
	"github.com/wintbiit/rmtv/internal/job"
	"github.com/wintbiit/rmtv/internal/rmbbs"
)

var modules = map[string]func() job.MessageProvider{
	"bilibili": func() job.MessageProvider {
		return bilibili.NewClient()
	},
	"rmbbs": func() job.MessageProvider {
		return rmbbs.NewClient()
	},
	"qflow": func() job.MessageProvider {
		return qflow.NewClient()
	},
}

func main() {
	interval, ok := os.LookupEnv("SCAN_INTERVAL")
	if !ok {
		interval = "10m"
	}

	dbpath, ok := os.LookupEnv("DB_PATH")
	if !ok {
		dbpath = "data/rmtv.db"
	}
	logrus.Infof("db path: %v", dbpath)

	enableModules, ok := os.LookupEnv("ENABLE_MODULES")
	if !ok {
		enableModules = "bilibili,rmbbs,qflow"
	}
	logrus.Infof("enabled modules: %v", enableModules)

	scanInterval, err := time.ParseDuration(interval)
	if err != nil {
		logrus.Fatalf("failed to parse scan interval: %v", err)
	}
	logrus.Infof("scan interval: %v", scanInterval)

	j := job.NewTvJob(
		job.WithScanInterval(scanInterval),
		job.WithDBPath(dbpath),
	)

	for _, module := range strings.Split(enableModules, ",") {
		if f, ok := modules[module]; ok {
			j = j.With(job.WithProvider(f()))
		}
	}

	if larkAppId, ok := os.LookupEnv("LARK_APP_ID"); ok {
		j = j.With(job.WithConsumer(lark.NewClient(larkAppId, os.Getenv("LARK_APP_SECRET"))))
	}

	if larkWebhooks, ok := os.LookupEnv("LARK_WEBHOOK_FILE"); ok {
		j = j.With(job.WithConsumer(lark.NewWebhookClient(larkWebhooks)))
	}

	if maxCountPerPush, ok := os.LookupEnv("MAX_COUNT_PER_PUSH"); ok {
		if count, err := strconv.Atoi(maxCountPerPush); err == nil && count > 0 {
			j = j.With(job.WithMaxCountPerPush(count))
		}
	}

	if err := j.Run(context.Background()); err != nil {
		logrus.Error(errors.Wrap(err, "failed to run job"))
	}
}
