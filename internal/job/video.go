package job

import (
	"context"

	"scutbot.cn/web/rmtv/internal/lark"

	"github.com/samber/lo"
	"github.com/sirupsen/logrus"
)

func (j *TvJob) onNewMessage(ctx context.Context, entries []lark.MessageEntry) error {
	logrus.Infof("Incoming %d new entries: %v", len(entries), lo.Map(entries, func(item lark.MessageEntry, _ int) string {
		return item.GetId()
	}))

	if j.maxCountPerPush > 0 && len(entries) > j.maxCountPerPush {
		entries = entries[:j.maxCountPerPush]
	}

	if j.larkClient != nil {
		logrus.Debug("Pushing new videos to Lark")
		if err := j.larkClient.PushMessage(ctx, entries); err != nil {
			logrus.Error("Failed to push message to Lark: ", err)
		}
	}
	return nil
}
