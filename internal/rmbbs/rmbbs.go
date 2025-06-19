package rmbbs

import (
	"net/http"
	"os"
	"time"

	"github.com/samber/lo"
	"github.com/samber/lo/parallel"
	"github.com/sirupsen/logrus"
	"go.uber.org/ratelimit"

	"resty.dev/v3"
	"scutbot.cn/web/rmtv/internal/lark"
	"scutbot.cn/web/rmtv/utils"
)

const (
	UA      = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/137.0.0.0 Safari/537.36"
	Referer = "https://bbs.robomaster.com/"
)

type Client struct {
	categories []string
	client     *resty.Client
}

type Response[T any] struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Success bool   `json:"success"`
	Data    T      `json:"data"`
}

func NewClient() *Client {
	cookiesRaw, ok := os.LookupEnv("RMBBS_COOKIES")
	if !ok {
		logrus.Fatalf("env variable COOKIES not set")
	}

	cookies, err := http.ParseCookie(cookiesRaw)
	if err != nil {
		logrus.Fatalf("failed to parse cookies: %v", err)
	}

	c := resty.New().
		SetBaseURL("https://bbs.robomaster.com/developers-server/rest/").
		SetRetryCount(3).
		SetRetryMaxWaitTime(5*1000).
		SetRetryWaitTime(1*1000).
		SetHeader("User-Agent", UA).
		SetHeader("Referer", Referer).
		SetDebug(utils.Debug).
		SetCookies(cookies).
		AddRequestMiddleware(limiter(ratelimit.New(3, ratelimit.Per(time.Minute))))

	return &Client{
		categories: []string{PostCategoryArticle},
		client:     c,
	}
}

func limiter(limiter ratelimit.Limiter) resty.RequestMiddleware {
	return func(client *resty.Client, req *resty.Request) error {
		limiter.Take()
		return nil
	}
}

func (c *Client) Collect() ([]lark.MessageEntry, error) {
	results := lo.Flatten(parallel.Map(c.categories, func(item string, index int) []lark.MessageEntry {
		result, err := c.ListPosts(item)
		if err != nil {
			logrus.Errorf("Failed to search videos with keyword %s: %v", item, err)
			return nil
		}
		return lo.Map(result, func(item ListPostsData, index int) lark.MessageEntry {
			return &item
		})
	}))

	results = lo.UniqBy(results, func(item lark.MessageEntry) string {
		return item.GetId()
	})

	return results, nil
}
