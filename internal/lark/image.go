package lark

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/pkg/errors"
)

const maxMessageImageBytes = 10 * 1000 * 1000

var mediaClient = &http.Client{Timeout: 15 * time.Second}

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

func uploadCardImage(ctx context.Context, raw string) (string, error) {
	if imageUploadClient == nil {
		return "", fmt.Errorf("image uploader unavailable")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return "", fmt.Errorf("invalid image address")
	}
	if u.Scheme == "http" && strings.HasSuffix(u.Hostname(), ".hdslb.com") {
		u.Scheme = "https"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", err
	}
	if strings.HasSuffix(u.Hostname(), ".hdslb.com") {
		req.Header.Set("Referer", "https://www.bilibili.com/")
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := mediaClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("image download returned HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxMessageImageBytes+1))
	if err != nil {
		return "", err
	}
	if len(data) > maxMessageImageBytes || !strings.HasPrefix(http.DetectContentType(data), "image/") {
		return "", fmt.Errorf("unsupported message image")
	}
	return imageUploadClient.uploadImage(ctx, bytes.NewReader(data))
}
