package main

import (
	"context"
	"os"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"scutbot.cn/web/rmtv/internal/job"
)

func main() {
	keywords, ok := os.LookupEnv("KEYWORDS")
	if !ok {
		keywords = "RoboMaster,机甲大师"
		logrus.Warnf("KEYWORDS environment variable not set. Using default keywords: %s", keywords)
	}

	j := job.NewTvJob(
		strings.Split(keywords, ","),
		job.WithLark(),
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
