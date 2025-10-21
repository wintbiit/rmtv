package rmbbs

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/pkg/errors"
	"github.com/samber/lo"
)

const (
	PostCategoryArticle = "ARTICLE"
	StatePass           = "PASS"
)

type ListPostsRequest struct {
	PageSize int `json:"pageSize"`
	PageNo   int `json:"pageNo"`
	Filter   struct {
		Category    string        `json:"category"`
		TagIds      []interface{} `json:"tagIds,omitempty"`
		SortByViews bool          `json:"sortByViews,omitempty"`
		Official    interface{}   `json:"official,omitempty"`
		Marrow      interface{}   `json:"marrow,omitempty"`
	} `json:"filter"`
}

type Tag struct {
	Id        int         `json:"id"`
	GroupName string      `json:"groupName"`
	Name      string      `json:"name"`
	HeadImg   interface{} `json:"headImg"`
}

type ListPostsData struct {
	History        bool        `json:"history"`
	Official       bool        `json:"official"`
	Top            bool        `json:"top"`
	Marrow         bool        `json:"marrow"`
	HeadImg        string      `json:"headImg"`
	Id             int         `json:"id"`
	Category       string      `json:"category"`
	CategoryDesc   string      `json:"categoryDesc"`
	Title          string      `json:"title"`
	Introduction   string      `json:"introduction"`
	AuthorId       int         `json:"authorId"`
	AuthorNickname string      `json:"authorNickname"`
	AuthorAvatar   string      `json:"authorAvatar"`
	CreateAt       time.Time   `json:"createAt"`
	Views          int         `json:"views"`
	Approvals      int         `json:"approvals"`
	Comments       int         `json:"comments"`
	Tags           []Tag       `json:"tags"`
	Solution       interface{} `json:"solution"`
	SolutionDesc   string      `json:"solutionDesc"`
	State          string      `json:"state"`
	StateDesc      string      `json:"stateDesc"`
	UpdateAt       time.Time   `json:"updateAt"`
	WikiId         interface{} `json:"wikiId"`
}

func (l *ListPostsData) GetType() string {
	return "RMBBS"
}

func (l *ListPostsData) GetTypeColor() string {
	return "lime"
}

func (l *ListPostsData) GetId() string {
	return fmt.Sprintf("rmbbs-%d", l.Id)
}

func (l *ListPostsData) GetPic() io.Reader {
	if len(l.HeadImg) == 0 {
		return nil
	}

	data, err := l.GetHeadImage()
	if err != nil {
		return nil
	}

	return io.NopCloser(bytes.NewReader(data))
}

func (l *ListPostsData) GetTitle() string {
	return l.Title
}

func (l *ListPostsData) GetDesc() string {
	return l.Introduction
}

func (l *ListPostsData) GetTags() []string {
	return lo.Map(l.Tags, func(item Tag, index int) string {
		return item.Name
	})
}

func (l *ListPostsData) GetPubDate() time.Time {
	return l.CreateAt
}

func (l *ListPostsData) GetAuthor() string {
	return l.AuthorNickname
}

func (l *ListPostsData) GetAuthorUrl() string {
	return fmt.Sprintf("https://bbs.robomaster.com/user/%d", l.AuthorId)
}

func (l *ListPostsData) GetUrl() string {
	return fmt.Sprintf("https://bbs.robomaster.com/article/%d", l.Id)
}

func (l *ListPostsData) GetAdditional() string {
	return fmt.Sprintf("<text_tag color='blue'>👀 %d</text_tag> "+
		"<text_tag color='green'>👍 %d</text_tag> "+
		"<text_tag color='red'>🗣️ %d</text_tag>",
		l.Views, l.Approvals, l.Comments)
}

func (l *ListPostsData) GetHeadImage() ([]byte, error) {
	type ImageData struct {
		Alt string `json:"alt"`
		Url string `json:"url"`
	}

	if len(l.HeadImg) == 0 {
		return nil, fmt.Errorf("no head image available")
	}

	var data []ImageData
	if err := json.Unmarshal([]byte(l.HeadImg), &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal head image: %w", err)
	}

	if len(data) == 0 || len(data[0].Url) == 0 {
		return nil, fmt.Errorf("no valid head image URL found")
	}

	imageData := data[0].Url
	if len(imageData) < 5 || imageData[:5] != "data:" {
		return nil, fmt.Errorf("head image URL is not a data URL")
	}

	imageData = imageData[len("data:image/png;base64,"):]
	return base64.StdEncoding.DecodeString(imageData)
}

type ListPostsResponse struct {
	List  []ListPostsData `json:"list"`
	Total int             `json:"total"`
	Size  int             `json:"size"`
}

func (c *Client) ListPosts(category string) ([]ListPostsData, error) {
	req := ListPostsRequest{
		PageNo:   1,
		PageSize: 10,
	}
	req.Filter.Category = category
	resp, err := c.client.R().
		SetBody(req).
		SetResult(Response[ListPostsResponse]{}).
		Post("/posts/list")
	if err != nil {
		return nil, err
	}

	if !resp.IsSuccess() {
		return nil, errors.New("failed to list posts: " + resp.String())
	}

	response := resp.Result().(*Response[ListPostsResponse])
	if response.Code != 0 {
		return nil, fmt.Errorf("failed to list posts: %d %s", response.Code, response.Message)
	}

	return lo.Filter(response.Data.List, func(item ListPostsData, index int) bool {
		return item.State == StatePass
	}), nil
}
