package job

import (
	"context"

	"github.com/samber/lo"
	"github.com/sirupsen/logrus"
	"scutbot.cn/web/rmtv/internal/bilibili"
)

func (j *TvJob) onNewVideos(ctx context.Context, video []bilibili.SearchResult) error {
	logrus.Infof("Incoming %d new videos: %v", len(video), lo.Map(video, func(item bilibili.SearchResult, _ int) string {
		return item.BVID
	}))

	if len(video) == 0 {
		logrus.Debug("No new videos found")
		return nil
	}

	if j.maxCountPerPush > 0 && len(video) > j.maxCountPerPush {
		video = video[:j.maxCountPerPush]
	}

	if j.larkClient != nil {
		logrus.Debug("Pushing new videos to Lark")
		if err := j.larkClient.PushMessage(ctx, video); err != nil {
			logrus.Error("Failed to push message to Lark: ", err)
		}
	}
	return nil
}
