package main

import (
	"context"
	"flag"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"scutbot.cn/web/rmtv/internal/qflow"

	"scutbot.cn/web/rmtv/internal/bilibili"
	"scutbot.cn/web/rmtv/internal/job"
	"scutbot.cn/web/rmtv/internal/rmbbs"
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
	interval := flag.Duration("interval", 10*time.Minute, "scan interval")
	dbpath := flag.String("db", "data/rmtv.db", "database path")
	enableModules := flag.String("modules", "bilibili,rmbbs,qflow", "modules to enable")
	flag.Parse()

	j := job.NewTvJob(
		job.WithLark(),
		job.WithScanInterval(*interval),
		job.WithDBPath(*dbpath),
	)

	for _, module := range strings.Split(*enableModules, ",") {
		if f, ok := modules[module]; ok {
			j = j.With(job.WithProvider(f()))
		}
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
