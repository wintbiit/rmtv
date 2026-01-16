package job

import (
	"context"
	"slices"

	"github.com/pkg/errors"
	"github.com/samber/lo"
	"github.com/samber/lo/parallel"
	"github.com/sirupsen/logrus"
	"github.com/wintbiit/rmtv/ent"
	"github.com/wintbiit/rmtv/ent/post"
)

type MessageProvider interface {
	Collect() ([]Post, error)
	Name() string
}

type MessageConsumer interface {
	PushMessage(ctx context.Context, videos []Post) error
}

func (j *TvJob) scan(ctx context.Context) error {
	logrus.Debug("Starting TV scan with providers: %+v", j.providers)

	tx, err := j.db.Tx(ctx)
	if err != nil {
		return errors.Wrapf(err, "failed to create transaction")
	}
	defer tx.Rollback()

	results := lo.Flatten(parallel.Map(j.providers, func(pri MessageProvider, index int) []Post {
		messages, err := pri.Collect()
		if err != nil {
			logrus.Errorf("Failed to collect results from provider %v", err)
			return nil
		}

		messages = lo.UniqBy(messages, func(item Post) string {
			return item.GetId()
		})

		latest, err := tx.Post.Query().
			Where(post.SourceEQ(pri.Name())).
			Order(ent.Desc(post.FieldPubDate)).
			Limit(1).
			First(ctx)
		if err != nil && !ent.IsNotFound(err) {
			logrus.Errorf("Failed to query latest post: %v", err)
			return nil
		}

		if latest != nil {
			messages = lo.Filter(messages, func(item Post, index int) bool {
				return item.GetPubDate().After(latest.PubDate)
			})
		}

		if err := tx.Post.CreateBulk(lo.Map(messages, func(item Post, index int) *ent.PostCreate {
			return tx.Post.Create().
				SetSource(pri.Name()).
				SetID(item.GetId()).
				SetNillablePicture(item.GetPic()).
				SetTitle(item.GetTitle()).
				SetTags(item.GetTags()).
				SetDescription(item.GetDesc()).
				SetPubDate(item.GetPubDate()).
				SetAuthor(item.GetAuthor()).
				SetAuthorURL(item.GetAuthorUrl()).
				SetURL(item.GetUrl()).
				SetExtra(item.GetExtra())
		})...).Exec(ctx); err != nil {
			logrus.Errorf("Failed to create posts: %v", err)
			return nil
		}

		logrus.Infof("%s found %d new videos", pri.Name(), len(messages))

		return messages
	}))

	results = lo.Filter(results, func(item Post, index int) bool {
		return item != nil
	})
	if len(results) == 0 {
		logrus.Infof("No new videos found")
		return nil
	}

	slices.SortFunc(results, func(a, b Post) int {
		return int(b.GetPubDate().Unix() - a.GetPubDate().Unix())
	})

	if err := j.onNewMessage(ctx, results); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return errors.Wrapf(err, "failed to commit transaction")
	}

	return nil
}
