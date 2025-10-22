package lark

import (
	"context"
	"io"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/pkg/errors"
)

func (c *Client) uploadImage(ctx context.Context, image io.Reader) (string, error) {
	req := larkim.NewCreateImageReqBuilder().
		Body(larkim.NewCreateImageReqBodyBuilder().
			ImageType(larkim.ImageTypeMessage).
			Image(image).
			Build()).
		Build()
	resp, err := c.client.Im.V1.Image.Create(ctx, req)
	if err != nil {
		return "", errors.Wrap(err, "lark uploadImage")
	}

	if !resp.Success() {
		return "", errors.Wrapf(resp, "lark uploadImage")
	}

	return *resp.Data.ImageKey, nil
}
