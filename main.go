package main

import (
	"context"
	"flag"
	"os"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"scutbot.cn/web/rmtv/internal/bilibili"
	"scutbot.cn/web/rmtv/internal/job"
	"scutbot.cn/web/rmtv/internal/rmbbs"
)

func main() {
	interval := flag.Duration("interval", 10*time.Minute, "scan interval")
	dbpath := flag.String("db", "data/rmtv.db", "database path")
	flag.Parse()

	j := job.NewTvJob(
		job.WithLark(),
		job.WithProvider(bilibili.NewClient()),
		job.WithProvider(rmbbs.NewClient()),
		job.WithScanInterval(*interval),
		job.WithDBPath(*dbpath),
	)

	if maxCountPerPush, ok := os.LookupEnv("MAX_COUNT_PER_PUSH"); ok {
		if count, err := strconv.Atoi(maxCountPerPush); err == nil && count > 0 {
			j = j.With(job.WithMaxCountPerPush(count))
		}
	}

	if err := j.Run(context.Background()); err != nil {
		logrus.Error(errors.Wrap(err, "failed to run job"))
	}
}
