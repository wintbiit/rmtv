package bilibili

import (
	"regexp"

	"github.com/pkg/errors"
	"github.com/samber/lo"
)

type SearchVideoResponse struct {
	Seid           string         `json:"seid"`
	Page           int            `json:"page"`
	Pagesize       int            `json:"pagesize"`
	NumResults     int            `json:"numResults"`
	NumPages       int            `json:"numPages"`
	SuggestKeyword string         `json:"suggest_keyword"`
	RqtType        string         `json:"rqt_type"`
	CostTime       CostTime       `json:"cost_time"`
	EggHit         int            `json:"egg_hit"`
	Result         []SearchResult `json:"result"`
	ShowColumn     int            `json:"show_column"`
}

type CostTime struct {
	ParamsCheck         string `json:"params_check"`
	IllegalHandler      string `json:"illegal_handler"`
	AsResponseFormat    string `json:"as_response_format"`
	AsRequest           string `json:"as_request"`
	SaveCache           string `json:"save_cache"`
	DeserializeResponse string `json:"deserialize_response"`
	AsRequestFormat     string `json:"as_request_format"`
	Total               string `json:"total"`
	MainHandler         string `json:"main_handler"`
}

const (
	SearchResultTypeVideo   = "video"
	SearchResultTypeArticle = "article"
)

type SearchResult struct {
	Type         string      `json:"type"`
	ID           int         `json:"id"`
	Author       string      `json:"author"`
	Mid          int         `json:"mid"`
	TypeID       string      `json:"typeid"`
	TypeName     string      `json:"typename"`
	ArcURL       string      `json:"arcurl"`
	Aid          int         `json:"aid"`
	BVID         string      `json:"bvid"`
	Title        string      `json:"title"`
	Description  string      `json:"description"`
	Pic          string      `json:"pic"`
	Play         int         `json:"play"`
	VideoReview  int         `json:"video_review"`
	Favorites    int         `json:"favorites"`
	Tag          string      `json:"tag"`
	Review       int         `json:"review"`
	PubDate      int         `json:"pubdate"`
	SendDate     int         `json:"senddate"`
	Duration     string      `json:"duration"`
	BadgePay     bool        `json:"badgepay"`
	HitColumns   []string    `json:"hit_columns"`
	ViewType     string      `json:"view_type"`
	IsPay        int         `json:"is_pay"`
	IsUnionVideo int         `json:"is_union_video"`
	RecTags      interface{} `json:"rec_tags"`
	NewRecTags   []string    `json:"new_rec_tags"`
	RankScore    int         `json:"rank_score"`
}

var titleRegex = regexp.MustCompile(`<em[^>]*>(.*?)</em>`)

func (s *SearchResult) postprocess() {
	s.Title = titleRegex.ReplaceAllString(s.Title, `**$1**`)
}

func (c *Client) SearchVideos(keyword string) ([]SearchResult, error) {
	resp, err := c.client.R().
		SetQueryParam("search_type", SearchResultTypeVideo).
		SetQueryParam("keyword", keyword).
		SetQueryParam("order", "pubdate").
		SetResult(Response[SearchVideoResponse]{}).
		Get("web-interface/wbi/search/type")
	if err != nil {
		return nil, errors.Wrap(err, "search videos error")
	}

	if !resp.IsSuccess() {
		return nil, errors.Errorf("search videos failed: %s", resp.String())
	}

	searchResp := resp.Result().(*Response[SearchVideoResponse])
	if searchResp.Code != 0 {
		return nil, errors.Errorf("search videos failed: %d %s", searchResp.Code, searchResp.Message)
	}

	return lo.Map(searchResp.Data.Result, func(item SearchResult, _ int) SearchResult {
		i := item
		i.postprocess()
		return i
	}), nil
}
