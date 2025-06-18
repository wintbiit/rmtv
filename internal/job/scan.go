package job

import (
	"context"
	"slices"

	"github.com/pkg/errors"
	"github.com/samber/lo"
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

	var results []bilibili.SearchResult
	for _, keyword := range j.KeywordList {
		result, err := j.bc.SearchVideos(keyword)
		if err != nil {
			return errors.Wrapf(err, "failed to search videos with keyword: %s", keyword)
		}

		results = slices.Concat(results, result)
	}

	results = lo.UniqBy(results, func(item bilibili.SearchResult) string {
		return item.BVID
	})

	if len(results) == 0 {
		return nil
	}

	return j.db.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(bucketName)
		if err != nil {
			return err
		}
		defer func() {
			latestTimeCursor := results[0].PubDate

			if err := bucket.Put(timeCursorKey, utils.MarshalInt(latestTimeCursor)); err != nil {
				logrus.Error("Failed to save latest time cursor: ", err)
			}
		}()

		timeCursor := utils.UnmarshalInt(bucket.Get(timeCursorKey))
		results := lo.Filter(results, func(item bilibili.SearchResult, _ int) bool {
			return item.PubDate > timeCursor
		})
		return j.onNewVideos(ctx, results)
	})
}
