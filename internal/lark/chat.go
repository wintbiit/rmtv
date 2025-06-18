package lark

import (
	"context"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/pkg/errors"
)

func (c *Client) ForeachChat(ctx context.Context, f func(*larkim.ListChat)) error {
	nextPageToken := ""

	for {
		resp, err := c.larkClient.Im.Chat.List(ctx, larkim.NewListChatReqBuilder().PageSize(20).PageToken(nextPageToken).Build())
		if err != nil {
			return errors.Wrap(err, "failed to list chats")
		}
		if !resp.Success() {
			return errors.Wrapf(resp, "failed to list chats")
		}

		for _, chat := range resp.Data.Items {
			f(chat)
		}

		if resp.Data.HasMore == nil || !*resp.Data.HasMore {
			break
		}

		nextPageToken = *resp.Data.PageToken
	}

	return nil
}
