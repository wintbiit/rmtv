package lark

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/samber/lo"
	"github.com/samber/lo/parallel"
	"github.com/sirupsen/logrus"
	"github.com/wintbiit/rmtv/internal/job"
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

const (
	templateId       = "AAqdTMBQENhuz"
	templateIdNoImg  = "AAqxTSf0s4wL9"
	imageKeyFallback = "img_v3_02nc_aa0dfc39-5024-4d47-a9a1-00d99a81a09g"
)

var imageUploadClient *Client

func BuildMessageCard(ctx context.Context, messages []job.MessageEntry) (*ChatCard, error) {
	images := parallel.Map(messages, func(item job.MessageEntry, i int) string {
		reader := item.GetPic()
		if reader == nil || imageUploadClient == nil {
			return imageKeyFallback
		}

		defer func() {
			if closer, ok := reader.(io.Closer); ok {
				err := closer.Close()
				if err != nil {
					logrus.Error(errors.Wrap(err, "failed to close image reader"))
				}
			}
		}()

		imageKey, err := imageUploadClient.uploadImage(ctx, item.GetPic())
		if err != nil {
			logrus.Error(errors.Wrap(err, "lark uploadImage"))
			return imageKeyFallback
		}

		return imageKey
	})

	template := templateId
	if len(lo.Filter(images, func(item string, _ int) bool {
		return item == imageKeyFallback
	})) > 0 {
		template = templateIdNoImg
	}

	var content ChatCard
	content.Data.TemplateId = template
	content.Type = "template"
	content.Data.TemplateVariable = map[string]interface{}{
		"count": strconv.Itoa(len(messages)),
		"object_img": lo.Map(messages, func(item job.MessageEntry, i int) map[string]interface{} {
			return map[string]interface{}{
				"img": map[string]interface{}{
					"img_key": images[i],
				},
				"title": fmt.Sprintf("<text_tag color='%s'>%s</text_tag> ", item.GetTypeColor(), item.GetType()) +
					fmt.Sprintf("**%s**", item.GetTitle()) +
					lo.Reduce(item.GetTags(), func(acc, tag string, _ int) string {
						return acc + "<text_tag color='blue'>" + tag + "</text_tag> "
					}, ""),
				"titleraw": item.GetTitle(),
				"senddate": time.Unix(item.GetPubDate().Unix(), 0).Format(time.DateTime),
				"url": map[string]string{
					"url": item.GetUrl(),
				},
				"author_url":  item.GetAuthorUrl(),
				"author":      item.GetAuthor(),
				"description": item.GetDesc(),
				"additional":  item.GetAdditional(),
				"type":        item.GetType(),
				"color":       item.GetTypeColor(),
			}
		}),
	}

	return &content, nil
}
