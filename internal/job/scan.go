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

type collectedPosts struct {
	source string
	posts  []Post
}

func (j *TvJob) scan(ctx context.Context) error {
	batches := parallel.Map(j.providers, func(provider MessageProvider, _ int) collectedPosts {
		source := provider.Name()
		messages, err := provider.Collect()
		if err != nil {
			logrus.WithError(err).WithField("source", source).Error("Failed to collect RMTV posts")
			return collectedPosts{source: source}
		}
		return collectedPosts{source: source, posts: lo.UniqBy(messages, func(item Post) string {
			return item.GetId()
		})}
	})

	tx, err := j.db.Tx(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create transaction")
	}
	defer tx.Rollback()

	// A transaction uses one database connection. Only collection runs concurrently.
	var results []Post
	for _, batch := range batches {
		if len(batch.posts) == 0 {
			logrus.Infof("%s found 0 new videos", batch.source)
			continue
		}
		latest, err := tx.Post.Query().
			Where(post.SourceEQ(batch.source)).
			Order(ent.Desc(post.FieldPubDate)).
			First(ctx)
		if err != nil && !ent.IsNotFound(err) {
			return errors.Wrapf(err, "failed to query latest post for %s", batch.source)
		}
		messages := batch.posts
		if latest != nil {
			messages = lo.Filter(messages, func(item Post, _ int) bool {
				return item.GetPubDate().After(latest.PubDate)
			})
		}
		if len(messages) > 0 {
			records := lo.Map(messages, func(item Post, _ int) *ent.PostCreate {
				return tx.Post.Create().
					SetSource(batch.source).
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
			})
			if err := tx.Post.CreateBulk(records...).Exec(ctx); err != nil {
				return errors.Wrapf(err, "failed to create posts for %s", batch.source)
			}
			results = append(results, messages...)
		}
		logrus.Infof("%s found %d new videos", batch.source, len(messages))
	}
	if err := tx.Commit(); err != nil {
		return errors.Wrap(err, "failed to commit transaction")
	}
	if len(results) == 0 {
		logrus.Info("No new videos found")
		return nil
	}
	slices.SortStableFunc(results, func(a, b Post) int {
		return b.GetPubDate().Compare(a.GetPubDate())
	})
	if err := j.onNewMessage(ctx, results); err != nil {
		return errors.Wrap(err, "posts stored but sending cards failed")
	}
	return nil
}
