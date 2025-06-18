package lark

import (
	"context"
	"strings"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/pkg/errors"
)

func (c *Client) uploadImage(ctx context.Context, url string) (string, error) {
	if !strings.HasPrefix(url, "http") {
		url = "https:" + url
	}
	image, err := c.client.R().Get(url)
	if err != nil {
		return "", errors.Wrap(err, "lark uploadImage")
	}

	if !image.IsSuccess() {
		return "", errors.Wrapf(err, "lark uploadImage: %s", image.String())
	}

	req := larkim.NewCreateImageReqBuilder().
		Body(larkim.NewCreateImageReqBodyBuilder().
			ImageType(larkim.ImageTypeMessage).
			Image(image.Body).
			Build()).
		Build()
	resp, err := c.larkClient.Im.V1.Image.Create(ctx, req)
	if err != nil {
		return "", errors.Wrap(err, "lark uploadImage")
	}

	if !resp.Success() {
		return "", errors.Wrapf(resp, "lark uploadImage")
	}

	return *resp.Data.ImageKey, nil
}
