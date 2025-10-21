package qflow

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	"go.uber.org/ratelimit"
	"resty.dev/v3"
	"scutbot.cn/web/rmtv/internal/lark"
	"scutbot.cn/web/rmtv/utils"
)

const (
	UA      = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/137.0.0.0 Safari/537.36"
	Referer = "https://qingflow.com"
)

type Client struct {
	client *resty.Client
}

func NewClient() *Client {
	cookiesRaw, ok := os.LookupEnv("QFLOW_COOKIES")
	if !ok {
		logrus.Fatalf("env variable COOKIES not set")
	}

	cookies, err := http.ParseCookie(cookiesRaw)
	if err != nil {
		logrus.Fatalf("failed to parse cookies: %v", err)
	}

	c := resty.New().
		SetRetryCount(3).
		SetRetryMaxWaitTime(5*1000).
		SetRetryWaitTime(1*1000).
		SetHeader("User-Agent", UA).
		SetHeader("Referer", Referer).
		SetHeader("Origin", "https://qingflow.com").
		SetDebug(utils.Debug).
		SetCookies(cookies).
		AddRequestMiddleware(limiter(ratelimit.New(3, ratelimit.Per(time.Minute))))

	logrus.Infof("Initialized QFlow client")

	return &Client{
		client: c,
	}
}

func limiter(limiter ratelimit.Limiter) resty.RequestMiddleware {
	return func(client *resty.Client, req *resty.Request) error {
		limiter.Take()
		return nil
	}
}

type Answer struct {
	ID          string    // 编号
	Status      string    // 流程状态
	University  string    // 学校名
	Team        string    // 队伍名
	Competition string    // 赛事类型
	Source      string    // 问题来源
	Question    string    // 问题
	Answer      string    // 回答
	CreatedAt   time.Time // 申请时间
	UpdatedAt   time.Time // 更新时间
}

func (m *Answer) GetType() string {
	return "轻流"
}

func (m *Answer) GetTypeColor() string {
	return "wathet"
}

func (m *Answer) GetId() string {
	return m.ID
}

func (m *Answer) GetPic() io.Reader {
	return nil
}

func (m *Answer) GetTitle() string {
	return m.Question
}

func (m *Answer) GetDesc() string {
	return m.Answer
}

func clearEng(s string) string {
	return strings.SplitN(s, " ", 2)[0]
}

func (m *Answer) GetTags() []string {
	return []string{
		clearEng(m.Competition),
		clearEng(m.Source),
		clearEng(m.Status),
	}
}

func (m *Answer) GetPubDate() time.Time {
	return m.UpdatedAt
}

func (m *Answer) GetAuthor() string {
	return fmt.Sprintf("%s-%s", m.University, m.Team)
}

func (m *Answer) GetAuthorUrl() string {
	return ""
}

func (m *Answer) GetUrl() string {
	return fmt.Sprintf("https://qingflow.com/appView/8n3ukbsnb001/shareView/8n3ukcuvb002?applyId=%s", m.ID)
}

func (m *Answer) GetAdditional() string {
	return ""
}

func (c *Client) Collect() ([]lark.MessageEntry, error) {
	resp, err := c.client.R().
		SetBody(`{"filter":{"pageSize":50,"pageNum":1,"type":8,"sorts":[{"queId":3,"queType":4,"isAscend":false}],"queries":[],"queryKey":null}}`).
		SetContentType("application/json").
		Post("https://qingflow.com/api/view/8n3ukcuvb002/apply/filter")
	if err != nil {
		return nil, err
	}

	if !resp.IsSuccess() {
		return nil, fmt.Errorf("failed to collect qflow: %d: %s", resp.StatusCode(), resp.String())
	}

	result := gjson.ParseBytes(resp.Bytes())
	if result.Get("errCode").Int() != 0 {
		return nil, fmt.Errorf("failed to collect qflow: %s", resp.String())
	}

	answers := make([]lark.MessageEntry, len(result.Get("data.list").Array()))
	for i, r := range result.Get("data.list").Array() {
		createdAt, err := time.Parse(time.DateTime, r.Get("answers.#(queId==2).values.0.value").String())
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse created at: %s", r.Get("answers.#(queId==2).values.0.value").String())
		}
		updatedAt, err := time.Parse(time.DateTime, r.Get("answers.#(queId==3).values.0.value").String())
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse updated at: %s", r.Get("answers.#(queId==3).values.0.value").String())
		}

		answers[i] = &Answer{
			ID:          r.Get("applyId").String(),
			Status:      r.Get("answers.#(queId==4).values.0.value").String(),
			University:  r.Get("answers.#(queId==286908413).values.0.value").String(),
			Team:        r.Get("answers.#(queId==286908414).values.0.value").String(),
			Competition: r.Get("answers.#(queId==286908418).values.0.value").String(),
			Source:      r.Get("answers.#(queId==286908419).values.0.value").String(),
			Question:    r.Get("answers.#(queId==286908421).values.0.value").String(),
			Answer:      r.Get("answers.#(queId==286908425).values.0.value").String(),
			CreatedAt:   createdAt,
			UpdatedAt:   updatedAt,
		}
	}

	return answers, nil
}
