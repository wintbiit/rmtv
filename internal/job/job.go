package job

import (
	"context"
	"os"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"go.etcd.io/bbolt"
	"scutbot.cn/web/rmtv/internal/lark"
)

type TvJob struct {
	providers       []MessageProvider
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

func WithProvider(p MessageProvider) TvJobOption {
	return func(j *TvJob) {
		j.providers = append(j.providers, p)
	}
}

func NewTvJob(options ...TvJobOption) *TvJob {
	job := &TvJob{
		scanInterval:    5 * time.Minute,
		dbPath:          "data/rmtv.db",
		maxCountPerPush: 10,
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
	var err error
	j.db, err = bbolt.Open(j.dbPath, 0o600, nil)
	if err != nil {
		return errors.Wrapf(err, "failed to open database at %s", j.dbPath)
	}
	defer j.db.Close()

	if err = j.scan(ctx); err != nil {
		return errors.Wrap(err, "initial scan failed")
	}

	for range time.Tick(j.scanInterval) {
		scanCtx, cancel := context.WithTimeout(ctx, j.scanInterval)
		if err := j.scan(scanCtx); err != nil {
			cancel()
			logrus.Error("Failed to scan TV: ", err)
		}
		cancel()
	}

	return nil
}
