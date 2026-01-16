package bilibili

import (
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/samber/lo"
	"github.com/samber/lo/parallel"
	"github.com/sirupsen/logrus"
	"github.com/wintbiit/rmtv/internal/job"
	"github.com/wintbiit/rmtv/utils"
	"go.uber.org/ratelimit"
	"resty.dev/v3"
)

const (
	UA      = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/137.0.0.0 Safari/537.36"
	Referer = "https://www.bilibili.com/"
)

type Client struct {
	client   *resty.Client
	keywords []string
}

const Module = "bilibili"

func (c *Client) Name() string {
	return Module
}

type Response[T any] struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

func NewClient() *Client {
	cookiesRaw, ok := os.LookupEnv("BILI_COOKIES")
	if !ok {
		logrus.Fatalf("env variable COOKIES not set")
	}

	cookies, err := http.ParseCookie(cookiesRaw)
	if err != nil {
		logrus.Fatalf("failed to parse cookies: %v", err)
	}

	keywords := "RoboMaster,机甲大师"
	keywordsOverride, ok := os.LookupEnv("BILI_KEYWORDS")
	if ok {
		keywords = keywordsOverride
	}

	c := resty.New().
		SetBaseURL("https://api.bilibili.com/x/").
		SetRetryCount(3).
		SetRetryMaxWaitTime(5*1000).
		SetRetryWaitTime(1*1000).
		SetHeader("User-Agent", UA).
		SetHeader("Referer", Referer).
		SetDebug(utils.Debug).
		SetCookies(cookies).
		AddRequestMiddleware(limiter(ratelimit.New(3, ratelimit.Per(time.Minute))))

	logrus.Infof("Initialized Bilibili client with keywords: %s", keywords)

	return &Client{
		client:   c,
		keywords: strings.Split(strings.ToLower(keywords), ","),
	}
}

func limiter(limiter ratelimit.Limiter) resty.RequestMiddleware {
	return func(client *resty.Client, req *resty.Request) error {
		limiter.Take()
		return nil
	}
}

func (c *Client) Collect() ([]job.Post, error) {
	results := lo.Flatten(parallel.Map(c.keywords, func(item string, index int) []job.Post {
		result, err := c.SearchVideos(item)
		if err != nil {
			logrus.Errorf("Failed to search videos with keyword %s: %v", item, err)
			return nil
		}
		result = lo.Filter(result, func(item SearchResult, index int) bool {
			return len(lo.Intersect(strings.Split(strings.ToLower(item.Tag), ","), c.keywords)) > 0
		})
		return lo.Map(result, func(item SearchResult, index int) job.Post {
			return &item
		})
	}))

	results = lo.UniqBy(results, func(item job.Post) string {
		return item.GetId()
	})

	return results, nil
}
