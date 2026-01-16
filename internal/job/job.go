package job

import (
	"context"
	errors2 "errors"
	"time"

	"github.com/pkg/errors"
	"github.com/samber/lo"
	"github.com/sirupsen/logrus"
	"github.com/wintbiit/rmtv/ent"

	_ "github.com/lib/pq"
)

type TvJob struct {
	providers       []MessageProvider
	consumers       []MessageConsumer
	dbUrl           string
	db              *ent.Client
	maxCountPerPush int
}

type TvJobOption func(*TvJob)

func WithDb(db string) TvJobOption {
	return func(j *TvJob) {
		j.dbUrl = db
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

func WithConsumer(c MessageConsumer) TvJobOption {
	return func(j *TvJob) {
		j.consumers = append(j.consumers, c)
	}
}

func NewTvJob(options ...TvJobOption) *TvJob {
	job := &TvJob{
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
	j.db, err = ent.Open("postgres", j.dbUrl)
	if err != nil {
		return errors.Wrap(err, "failed to open db")
	}
	defer j.db.Close()
	if err := j.db.Schema.Create(ctx); err != nil {
		return errors.Wrap(err, "failed to create schema")
	}

	if err = j.scan(ctx); err != nil {
		return errors.Wrap(err, "initial scan failed")
	}

	return nil
}

type PostExtra interface {
	String() string
}

type Post interface {
	GetType() string
	GetTypeColor() string
	GetId() string
	GetPic() *string
	GetTitle() string
	GetDesc() string
	GetTags() []string
	GetPubDate() time.Time
	GetAuthor() string
	GetAuthorUrl() string
	GetUrl() string
	GetExtra() PostExtra
}

func (j *TvJob) onNewMessage(ctx context.Context, entries []Post) error {
	logrus.Infof("Incoming %d new entries: %v", len(entries), lo.Map(entries, func(item Post, _ int) string {
		return item.GetId()
	}))

	if j.maxCountPerPush > 0 && len(entries) > j.maxCountPerPush {
		entries = entries[:j.maxCountPerPush]
	}

	errs := make([]error, 0, len(j.consumers))
	for _, consumer := range j.consumers {
		if err := consumer.PushMessage(ctx, entries); err != nil {
			errs = append(errs, errors.Wrap(err, "failed to push message"))
		}
	}

	if err := errors2.Join(errs...); err != nil {
		return err
	}

	logrus.Infof("pushed %d messages to %d consumers", len(entries), len(j.consumers))

	return nil
}
