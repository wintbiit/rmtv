package job

import (
	"context"
	"os"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"go.etcd.io/bbolt"
	"scutbot.cn/web/rmtv/internal/bilibili"
	"scutbot.cn/web/rmtv/internal/lark"
)

type TvJob struct {
	KeywordList []string

	bc *bilibili.Client

	scanInterval    time.Duration
	larkClient      *lark.Client
	dbPath          string
	db              *bbolt.DB
	maxCountPerPush int
}

type TvJobOption func(*TvJob)

func WithLark() TvJobOption {
	return func(j *TvJob) {
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

		j.larkClient = lark.NewClient(&lark.Config{
			AppId:           larkClientId,
			AppSecret:       larkClientSecret,
			WebhookFilePath: webhookFilePath,
		})
	}
}

func WithScanInterval(interval time.Duration) TvJobOption {
	return func(j *TvJob) {
		j.scanInterval = interval
	}
}

func WithDBPath(path string) TvJobOption {
	return func(j *TvJob) {
		j.dbPath = path
	}
}

func WithMaxCountPerPush(count int) TvJobOption {
	return func(j *TvJob) {
		if count <= 0 {
			logrus.Fatal("maxCountPerPush must be greater than 0")
		}
		j.maxCountPerPush = count
	}
}

func NewTvJob(keywords []string, options ...TvJobOption) *TvJob {
	if len(keywords) == 0 {
		logrus.Fatal("keywords is empty")
	}

	job := &TvJob{
		KeywordList:  keywords,
		bc:           bilibili.NewClient(),
		scanInterval: 5 * time.Minute,
		dbPath:       "data/rmtv.db",
	}

	for _, option := range options {
		option(job)
	}

	return job
}

func (j *TvJob) With(options ...TvJobOption) *TvJob {
	for _, option := range options {
		option(j)
	}
	return j
}

func (j *TvJob) Run(ctx context.Context) error {
	ticker := time.NewTicker(j.scanInterval)
	defer ticker.Stop()
	var err error
	j.db, err = bbolt.Open(j.dbPath, 0o600, nil)
	if err != nil {
		return errors.Wrapf(err, "failed to open database at %s", j.dbPath)
	}
	defer j.db.Close()

	if err = j.scan(ctx); err != nil {
		return errors.Wrap(err, "initial scan failed")
	}

	for range ticker.C {
		scanCtx, cancel := context.WithTimeout(ctx, j.scanInterval)
		if err := j.scan(scanCtx); err != nil {
			cancel()
			logrus.Error("Failed to scan TV: ", err)
		}
		cancel()
	}

	return nil
}
