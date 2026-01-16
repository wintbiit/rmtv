package lark

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
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
	templateId      = "AAqdTMBQENhuz"
	templateIdNoImg = "AAqxTSf0s4wL9"
)

var imageUploadClient *Client

func BuildMessageCard(ctx context.Context, messages []job.Post) (*ChatCard, error) {
	images := parallel.Map(messages, func(item job.Post, i int) string {
		url := item.GetPic()
		if url == nil || imageUploadClient == nil {
			return ""
		}

		r, err := http.Get(*url)
		if err != nil {
			logrus.Error(errors.Wrap(err, "failed to get image url"))
			return ""
		}
		defer r.Body.Close()

		imageKey, err := imageUploadClient.uploadImage(ctx, r.Body)
		if err != nil {
			logrus.Error(errors.Wrap(err, "lark uploadImage"))
			return ""
		}

		return imageKey
	})

	template := templateId
	if lo.EveryBy(images, func(item string) bool {
		return item == ""
	}) {
		template = templateIdNoImg
	}

	var content ChatCard
	content.Data.TemplateId = template
	content.Type = "template"
	content.Data.TemplateVariable = map[string]interface{}{
		"count": strconv.Itoa(len(messages)),
		"object_img": lo.Map(messages, func(item job.Post, i int) map[string]interface{} {
			var title strings.Builder
			title.WriteString(fmt.Sprintf("<text_tag color='%s'>%s</text_tag> ", item.GetTypeColor(), item.GetType()))
			if !strings.Contains(item.GetTitle(), "\n") {
				title.WriteString(fmt.Sprintf("**%s**", item.GetTitle()))
			} else {
				title.WriteString(item.GetTitle())
			}
			title.WriteString(lo.Reduce(item.GetTags(), func(acc, tag string, _ int) string {
				return acc + "<text_tag color='blue'>" + tag + "</text_tag> "
			}, ""))

			return map[string]interface{}{
				"img": map[string]interface{}{
					"img_key": images[i],
				},
				"title":    title.String(),
				"titleraw": item.GetTitle(),
				"senddate": time.Unix(item.GetPubDate().Unix(), 0).Format(time.DateTime),
				"url": map[string]string{
					"url": item.GetUrl(),
				},
				"author_url":  item.GetAuthorUrl(),
				"author":      item.GetAuthor(),
				"description": item.GetDesc(),
				"additional":  item.GetExtra(),
				"type":        item.GetType(),
				"color":       item.GetTypeColor(),
			}
		}),
	}

	return &content, nil
}
