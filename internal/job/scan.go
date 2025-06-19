package job

import (
	"context"
	"slices"

	"github.com/pkg/errors"
	"github.com/samber/lo"
	"github.com/samber/lo/parallel"
	"github.com/sirupsen/logrus"
	"go.etcd.io/bbolt"
	"scutbot.cn/web/rmtv/internal/bilibili"
	"scutbot.cn/web/rmtv/utils"
)

var (
	bucketName    = []byte("rmtv")
	timeCursorKey = []byte("time_cursor")
)

func (j *TvJob) scan(ctx context.Context) error {
	logrus.Debug("Starting TV scan with keywords: ", j.KeywordList)

	results := lo.Flatten(parallel.Map(j.KeywordList, func(item string, index int) []bilibili.SearchResult {
		result, err := j.bc.SearchVideos(item)
		if err != nil {
			logrus.Errorf("Failed to search videos with keyword %s: %v", item, err)
			return nil
		}
		return result
	}))

	results = lo.UniqBy(results, func(item bilibili.SearchResult) string {
		return item.BVID
	})
	slices.SortFunc(results, func(a, b bilibili.SearchResult) int {
		return b.PubDate - a.PubDate
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

		timeCursor := utils.UnmarshalInt(bucket.Get(timeCursorKey))
		results := lo.Filter(results, func(item bilibili.SearchResult, _ int) bool {
			return item.PubDate > timeCursor
		})
		if len(results) == 0 {
			logrus.Debug("No new videos found")
			return nil
		}

		if err = j.onNewVideos(ctx, results); err != nil {
			return errors.Wrapf(err, "failed to save new videos")
		}

		latestTimeCursor := results[0].PubDate

		if err = bucket.Put(timeCursorKey, utils.MarshalInt(latestTimeCursor)); err != nil {
			return errors.Wrapf(err, "failed to update time cursor")
		}

		logrus.Infof("Updated time cursor to %d", latestTimeCursor)
		return nil
	})
}
