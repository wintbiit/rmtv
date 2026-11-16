package lark

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/wintbiit/rmtv/internal/job"
)

const maxCardBytes = 20000
const maxCardComponents = 200
const maxMessageImageBytes = 10 * 1000 * 1000

type element map[string]any

type ChatCard struct {
	Schema   string  `json:"schema"`
	Config   element `json:"config"`
	Body     element `json:"body"`
	identity string
}

type ChatContent struct {
	MsgType string    `json:"msg_type"`
	Card    *ChatCard `json:"card"`
}

type cardImage struct {
	Key, Caption string
}

type cardEntry struct {
	job.Item
	pictures []cardImage
}

type cardTheme struct {
	label, tag, tint, light, dark string
}

var themes = map[string]cardTheme{
	"Bilibili": {"B 站", "wathet", "rmtv_bili", "rgba(38,114,210,0.07)", "rgba(78,149,245,0.10)"},
	"RMBBS":    {"论坛", "blue", "rmtv_forum", "rgba(94,112,137,0.08)", "rgba(132,153,185,0.12)"},
	"轻流":       {"轻流", "turquoise", "rmtv_qflow", "rgba(0,133,112,0.08)", "rgba(41,180,158,0.12)"},
}

var cardLocation = time.FixedZone("CST", 8*60*60)
var markdownEscaper = strings.NewReplacer("\\", "\\\\", "[", "\\[", "]", "\\]", "*", "\\*", "_", "\\_", "`", "\\`", "<", "&lt;", ">", "&gt;")
var mediaClient = &http.Client{Timeout: 15 * time.Second}
var imageUploadClient *Client

func BuildMessageCard(ctx context.Context, posts []job.Post) (*ChatCard, error) {
	cards, err := BuildMessageCards(ctx, posts)
	if err != nil {
		return nil, err
	}
	if len(cards) != 1 {
		return nil, fmt.Errorf("expected one card, got %d", len(cards))
	}
	return cards[0], nil
}

func BuildMessageCards(ctx context.Context, posts []job.Post) ([]*ChatCard, error) {
	items := make([]job.Item, 0, len(posts))
	for _, post := range posts {
		items = append(items, job.ItemFromPost(post))
	}
	return RenderCards(ctx, items)
}

func RenderCards(ctx context.Context, items []job.Item) ([]*ChatCard, error) {
	items = slices.Clone(items)
	slices.SortStableFunc(items, func(a, b job.Item) int { return b.PublishedAt.Compare(a.PublishedAt) })
	entries := make([]cardEntry, 0, len(items))
	keys := make(map[string]string)
	for _, item := range items {
		entry := cardEntry{Item: item}
		for _, image := range item.Images {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			key, seen := keys[image.URL]
			if !seen {
				var err error
				key, err = uploadCardImage(ctx, image.URL)
				if err != nil {
					logrus.WithField("item_id", item.ID).Warn("Could not attach RMTV image")
				}
				keys[image.URL] = key
			}
			if key != "" {
				entry.pictures = append(entry.pictures, cardImage{key, image.Caption})
			}
		}
		entries = append(entries, entry)
	}
	return packCards(entries)
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

func plain(text string) element    { return element{"tag": "plain_text", "content": text} }
func markdown(text string) element { return element{"tag": "markdown", "content": text} }
func shorten(text string, max int) string {
	chars := []rune(text)
	if len(chars) <= max {
		return text
	}
	return strings.TrimSpace(string(chars[:max])) + "…"
}

func imageNode(image cardImage, thumbnail bool) element {
	node := element{"tag": "img", "img_key": image.Key, "alt": plain(image.Caption), "preview": true, "scale_type": "fit_horizontal"}
	if thumbnail {
		node["size"] = "88px 50px"
		node["scale_type"] = "crop_center"
	}
	return node
}

func gallery(pictures []cardImage) []element {
	var nodes []element
	for i := 0; i < len(pictures); {
		picture := pictures[i]
		if picture.Caption != "" {
			nodes = append(nodes, markdown(markdownEscaper.Replace(picture.Caption)), imageNode(picture, false))
			i++
			continue
		}
		end := i + 1
		for end < len(pictures) && end-i < 3 && pictures[end].Caption == "" {
			end++
		}
		if end-i == 1 {
			nodes = append(nodes, imageNode(picture, false))
		} else {
			mode := "double"
			if end-i == 3 {
				mode = "triple"
			}
			images := make([]element, 0, end-i)
			for _, p := range pictures[i:end] {
				images = append(images, element{"img_key": p.Key})
			}
			nodes = append(nodes, element{"tag": "img_combination", "combination_mode": mode, "img_list": images})
		}
		i = end
	}
	return nodes
}

func renderEntry(entry cardEntry, full bool) element {
	theme, ok := themes[entry.Source]
	if !ok {
		theme = cardTheme{label: entry.Source, tag: "blue"}
	}
	title := shorten(strings.Join(strings.Fields(entry.Title), " "), 64)
	if title == "" {
		title = "查看原文"
	}
	text := markdownEscaper.Replace(title)
	if u, err := url.Parse(entry.URL); err == nil && u.Host != "" && (u.Scheme == "https" || u.Scheme == "http") {
		link := strings.NewReplacer("(", "%28", ")", "%29", " ", "%20").Replace(u.String())
		text = "[" + text + "](" + link + ")"
	}
	meta := []string{}
	if entry.Number != "" {
		meta = append(meta, "#"+entry.Number)
	} else if entry.Author != "" {
		meta = append(meta, shorten(entry.Author, 24))
	}
	if !entry.PublishedAt.IsZero() {
		meta = append(meta, entry.PublishedAt.In(cardLocation).Format("01-02 15:04"))
	}
	label := fmt.Sprintf("<text_tag color='%s'>%s</text_tag>", theme.tag, markdownEscaper.Replace(theme.label))
	if len(meta) > 0 {
		label += "  <font color='grey'>" + markdownEscaper.Replace(strings.Join(meta, " · ")) + "</font>"
	}
	top := []element{{"tag": "div", "text": element{"tag": "lark_md", "content": text, "lines": 2, "text_size": "rmtv_title"}}, markdown(label)}

	top[1]["text_size"] = "rmtv_meta"
	if len(entry.pictures) > 0 {
		top = []element{{"tag": "column_set", "horizontal_spacing": "8px", "columns": []element{
			{"tag": "column", "width": "weighted", "weight": 1, "vertical_spacing": "4px", "elements": top},
			{"tag": "column", "width": "88px", "vertical_align": "center", "elements": []element{imageNode(entry.pictures[0], true)}},
		}}}
	}
	if full && (entry.Body != "" || len(entry.pictures) > 0 || title != entry.Title) {
		details := []element{markdown("**" + markdownEscaper.Replace(entry.Title) + "**")}
		if entry.Body != "" {
			details = append(details, markdown(markdownEscaper.Replace(entry.Body)))
		}
		details = append(details, gallery(entry.pictures)...)
		caption := "查看全文"
		if len(entry.pictures) > 0 {
			caption += fmt.Sprintf(" · %d 张图", len(entry.pictures))
		}
		top = append(top, element{"tag": "collapsible_panel", "expanded": false, "vertical_spacing": "6px",
			"header": element{"title": plain(caption)}, "elements": details})
	}
	section := element{"tag": "column_set", "columns": []element{
		{"tag": "column", "width": "weighted", "weight": 1, "vertical_spacing": "6px", "padding": "8px 10px 8px 10px", "elements": top},
	}, "margin": "0px 0px 0px 0px"}
	if theme.tint != "" {
		section["background_style"] = theme.tint
	}
	return section
}

func renderPage(entries []cardEntry, nodes []element, suffix string) *ChatCard {
	counts := map[string]int{}
	identities := make([]string, 0, len(entries))
	for _, entry := range entries {
		counts[entry.Source]++
		identities = append(identities, entry.Source+"\x00"+entry.ID+"\x00"+entry.PublishedAt.UTC().Format(time.RFC3339Nano))
	}
	summary := fmt.Sprintf("RMTV · %d 条更新", len(entries))
	if len(entries) == 1 {
		label := themes[entries[0].Source].label
		if label == "" {
			label = entries[0].Source
		}
		summary = label + " · " + shorten(strings.Join(strings.Fields(entries[0].Title), " "), 70)
	} else {
		for _, source := range []string{"Bilibili", "轻流", "RMBBS"} {
			if count := counts[source]; count > 0 {
				summary += fmt.Sprintf(" · %s %d", themes[source].label, count)
			}
		}
	}
	colors := element{}
	for _, theme := range themes {
		colors[theme.tint] = element{"light_mode": theme.light, "dark_mode": theme.dark}
	}
	if suffix != "" {
		nodes = append(slices.Clone(nodes), markdown("<font color='grey'>"+strings.TrimPrefix(suffix, " · ")+"</font>"))
	}
	return &ChatCard{
		Schema: "2.0",
		Config: element{"update_multi": true, "summary": element{"content": summary + suffix}, "style": element{
			"color": colors,
			"text_size": element{
				"rmtv_title": element{"default": "normal", "pc": "heading", "mobile": "normal"},
				"rmtv_meta":  element{"default": "notation", "pc": "notation", "mobile": "notation"},
			},
		}},
		Body:     element{"padding": "8px 8px 8px 8px", "vertical_spacing": "6px", "elements": nodes},
		identity: strings.Join(identities, "\n"),
	}
}

func componentCount(nodes []element) int {
	count := len(nodes)
	for _, node := range nodes {
		for _, key := range []string{"elements", "columns"} {
			if children, ok := node[key].([]element); ok {
				count += componentCount(children)
			}
		}
	}
	return count
}

func fitsCard(card *ChatCard) bool {
	if componentCount(card.Body["elements"].([]element)) > maxCardComponents {
		return false
	}
	webhook, _ := json.Marshal(ChatContent{MsgType: "interactive", Card: card})
	app, _ := json.Marshal(card)
	envelope, _ := json.Marshal(map[string]string{
		"receive_id": strings.Repeat("x", 128), "msg_type": "interactive", "content": string(app), "uuid": strings.Repeat("x", 36),
	})
	return len(webhook) <= maxCardBytes && len(envelope) <= maxCardBytes
}

func packCards(entries []cardEntry) ([]*ChatCard, error) {
	type page struct {
		entries []cardEntry
		nodes   []element
	}
	var pages []page
	var current page
	const pageAllowance = " · 999999/999999"
	for _, entry := range entries {
		section := renderEntry(entry, true)
		if !fitsCard(renderPage([]cardEntry{entry}, []element{section}, pageAllowance)) {
			section = renderEntry(entry, false)
		}
		proposed := page{append(slices.Clone(current.entries), entry), append(slices.Clone(current.nodes), section)}
		if len(current.nodes) > 0 && !fitsCard(renderPage(proposed.entries, proposed.nodes, pageAllowance)) {
			pages = append(pages, current)
			proposed = page{[]cardEntry{entry}, []element{section}}
		}
		if !fitsCard(renderPage(proposed.entries, proposed.nodes, pageAllowance)) {
			return nil, fmt.Errorf("RMTV item %s exceeds card limits", entry.ID)
		}
		current = proposed
	}
	if len(current.nodes) > 0 {
		pages = append(pages, current)
	}
	cards := make([]*ChatCard, 0, len(pages))
	for i, page := range pages {
		suffix := ""
		if len(pages) > 1 {
			suffix = fmt.Sprintf(" · %d/%d", i+1, len(pages))
		}
		card := renderPage(page.entries, page.nodes, suffix)
		if !fitsCard(card) {
			return nil, fmt.Errorf("RMTV card %d exceeds send limits", i+1)
		}
		cards = append(cards, card)
	}
	return cards, nil
}
