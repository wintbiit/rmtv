package lark

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	"github.com/samber/lo/parallel"
	"github.com/sirupsen/logrus"
	"scutbot.cn/web/rmtv/internal/bilibili"
	"strconv"
	"strings"
	"time"
)

type ChatContent struct {
	MsgType string    `json:"msg_type"`
	Card    *ChatCard `json:"card"`
}

type ChatCard struct {
	Type string `json:"type"`
	Data struct {
		TemplateId       string                 `json:"template_id"`
		TemplateVariable map[string]interface{} `json:"template_variable"`
	} `json:"data"`
}

const templateId = "AAqdTMBQENhuz"
const imageKeyFallback = "img_v3_02nc_aa0dfc39-5024-4d47-a9a1-00d99a81a09g"

func (c *Client) buildMessageCard(ctx context.Context, videos []bilibili.SearchResult) (*ChatCard, error) {
	images := parallel.Map(videos, func(item bilibili.SearchResult, i int) string {
		imageKey, err := c.uploadImage(ctx, item.Pic)
		if err != nil {
			logrus.Error(errors.Wrap(err, "lark uploadImage"))
			return imageKeyFallback
		}

		return imageKey
	})

	var content ChatCard
	content.Data.TemplateId = templateId
	content.Type = "template"
	content.Data.TemplateVariable = map[string]interface{}{
		"count": strconv.Itoa(len(videos)),
		"object_img": lo.Map(videos, func(item bilibili.SearchResult, i int) map[string]interface{} {
			return map[string]interface{}{
				"img": map[string]interface{}{
					"img_key": images[i],
				},
				"title": item.Title + strings.Join(lo.Map(strings.Split(item.Tag, ","), func(item string, index int) string {
					return fmt.Sprintf("<text_tag color='blue'>%s</text_tag> ", item)
				}), ""),
				"senddate": time.Unix(int64(item.PubDate), 0).Format(time.DateTime),
				"duration": item.Duration,
				"url": map[string]string{
					"url": fmt.Sprintf("https://b23.tv/%s", item.BVID),
				},
				"author_url":  fmt.Sprintf("https://space.bilibili.com/%d", item.Mid),
				"author":      item.Author,
				"description": item.Description,
				"titleraw":    item.Title,
			}
		}),
	}

	return &content, nil
}
