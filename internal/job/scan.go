package job

import (
	"context"
	"slices"

	"github.com/pkg/errors"
	"github.com/samber/lo"
	"github.com/samber/lo/parallel"
	"github.com/sirupsen/logrus"
	"go.etcd.io/bbolt"
	"scutbot.cn/web/rmtv/internal/lark"
	"scutbot.cn/web/rmtv/utils"
)

var (
	bucketName    = []byte("rmtv")
	timeCursorKey = []byte("time_cursor")
)

type MessageProvider interface {
	Collect() ([]lark.MessageEntry, error)
}

func (j *TvJob) scan(ctx context.Context) error {
	logrus.Debug("Starting TV scan with providers: %+v", j.providers)

	results := lo.Flatten(parallel.Map(j.providers, func(item MessageProvider, index int) []lark.MessageEntry {
		messages, err := item.Collect()
		if err != nil {
			logrus.Errorf("Failed to collect results from provider %v", err)
			return nil
		}

		return messages
	}))

	slices.SortFunc(results, func(a, b lark.MessageEntry) int {
		return int(b.GetPubDate().Unix() - a.GetPubDate().Unix())
	})

	if len(results) == 0 {
		logrus.Debug("No new videos found")
		return nil
	}

	return j.db.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(bucketName)
		if err != nil {
			return err
		}

		timeCursor := utils.UnmarshalInt64(bucket.Get(timeCursorKey))
		results := lo.Filter(results, func(item lark.MessageEntry, _ int) bool {
			return item.GetPubDate().Unix() > timeCursor
		})
		if len(results) == 0 {
			logrus.Debug("No new messages found")
			return nil
		}

		if err = j.onNewMessage(ctx, results); err != nil {
			return errors.Wrapf(err, "failed to save new videos")
		}

		latestTimeCursor := results[0].GetPubDate().Unix()

		if err = bucket.Put(timeCursorKey, utils.MarshalInt64(latestTimeCursor)); err != nil {
			return errors.Wrapf(err, "failed to update time cursor")
		}

		logrus.Infof("Updated time cursor to %d", latestTimeCursor)
		return nil
	})
}
