package lark

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/wintbiit/rmtv/internal/job"
)

const maxCardBytes = 20000
const maxCardComponents = 200
const maxInlineBodyRunes = 48

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
	AfterTitle   bool
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
var imageUploadClient *Client

func BuildMessageCard(ctx context.Context, posts []job.Post) (*ChatCard, error) {
	cards, err := buildMessageCards(ctx, posts, 0)
	if err != nil {
		return nil, err
	}
	if len(cards) != 1 {
		return nil, fmt.Errorf("expected one card, got %d", len(cards))
	}
	return cards[0], nil
}

func buildMessageCards(ctx context.Context, posts []job.Post, maxCards int) ([]*ChatCard, error) {
	items := make([]job.Item, 0, len(posts))
	for _, post := range posts {
		items = append(items, job.ItemFromPost(post))
	}
	return renderCards(ctx, items, maxCards)
}

func RenderCards(ctx context.Context, items []job.Item) ([]*ChatCard, error) {
	return renderCards(ctx, items, 0)
}

func renderCards(ctx context.Context, items []job.Item, maxCards int) ([]*ChatCard, error) {
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
				entry.pictures = append(entry.pictures, cardImage{Key: key, Caption: image.Caption, AfterTitle: image.AfterTitle})
			}
		}
		entries = append(entries, entry)
	}
	return packCards(entries, maxCards)
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
	titleText := strings.Join(strings.Fields(entry.Title), " ")
	title := shorten(titleText, 64)
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
	chip := fmt.Sprintf("<text_tag color='%s'>%s</text_tag>", theme.tag, markdownEscaper.Replace(theme.label))
	top := []element{{"tag": "div", "text": element{
		"tag": "lark_md", "content": chip + "  " + text, "lines": 2, "text_size": "rmtv_title",
	}}}
	if len(meta) > 0 {
		top = append(top, element{"tag": "markdown", "text_size": "rmtv_meta",
			"content": "<font color='grey'>" + markdownEscaper.Replace(strings.Join(meta, " · ")) + "</font>"})
	}
	if len(entry.pictures) > 0 {
		top = []element{{"tag": "column_set", "horizontal_spacing": "8px", "columns": []element{
			{"tag": "column", "width": "weighted", "weight": 1, "vertical_spacing": "4px", "elements": top},
			{"tag": "column", "width": "88px", "vertical_align": "center", "elements": []element{imageNode(entry.pictures[0], true)}},
		}}}
	}
	entry.Body = strings.TrimSpace(entry.Body)
	if entry.Body == "-" || entry.Body == "—" {
		entry.Body = ""
	}
	shortBody := len([]rune(entry.Body)) <= maxInlineBodyRunes && !strings.ContainsAny(entry.Body, "\r\n")
	detailImages := len(entry.pictures) > 1 || (len(entry.pictures) == 1 && entry.pictures[0].Caption != "")
	if full && (title != titleText || !shortBody || detailImages) {
		top = append(top, element{"tag": "collapsible_panel", "expanded": false, "vertical_spacing": "6px",
			"header": element{"title": plain("查看全文")}, "elements": renderDetails(entry)})
	} else if shortBody && entry.Body != "" {
		body := markdownEscaper.Replace(entry.Body)
		if entry.Source == "轻流" {
			body = "**答复**　" + body
		}
		top = append(top, markdown(body))
	}
	section := element{"tag": "column_set", "columns": []element{
		{"tag": "column", "width": "weighted", "weight": 1, "vertical_spacing": "6px", "padding": "8px 10px 8px 10px", "elements": top},
	}, "margin": "0px 0px 0px 0px"}
	if theme.tint != "" {
		section["background_style"] = theme.tint
	}
	return section
}

func renderDetails(entry cardEntry) []element {
	title := "**" + markdownEscaper.Replace(entry.Title) + "**"
	body := markdownEscaper.Replace(entry.Body)
	if entry.Source == "轻流" {
		title = "**问题**<br>" + markdownEscaper.Replace(entry.Title)
		if body != "" {
			body = "**答复**<br>" + body
		}
	}
	var afterTitle, afterBody []cardImage
	for _, picture := range entry.pictures {
		if picture.AfterTitle {
			afterTitle = append(afterTitle, picture)
		} else {
			afterBody = append(afterBody, picture)
		}
	}
	nodes := append([]element{markdown(title)}, gallery(afterTitle)...)
	if body != "" {
		nodes = append(nodes, markdown(body))
	}
	return append(nodes, gallery(afterBody)...)
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

func packCards(entries []cardEntry, maxCards int) ([]*ChatCard, error) {
	type page struct {
		entries []cardEntry
		nodes   []element
	}
	var pages []page
	var current page
	const pageAllowance = " · 9999999999999999999/9999999999999999999 · 另有 9999999999999999999 条未展示"
	for _, entry := range entries {
		section := renderEntry(entry, true)
		proposed := page{append(slices.Clone(current.entries), entry), append(slices.Clone(current.nodes), section)}
		if fitsCard(renderPage(proposed.entries, proposed.nodes, pageAllowance)) {
			current = proposed
			continue
		}
		if len(current.nodes) > 0 {
			pages = append(pages, current)
			if maxCards > 0 && len(pages) == maxCards {
				current = page{}
				break
			}
		}
		proposed = page{[]cardEntry{entry}, []element{section}}
		if !fitsCard(renderPage(proposed.entries, proposed.nodes, pageAllowance)) {
			proposed.nodes[0] = renderEntry(entry, false)
			if !fitsCard(renderPage(proposed.entries, proposed.nodes, pageAllowance)) {
				return nil, fmt.Errorf("RMTV item %s exceeds card limits", entry.ID)
			}
		}
		current = proposed
	}
	if len(current.nodes) > 0 {
		pages = append(pages, current)
	}
	unshown := len(entries)
	for _, page := range pages {
		unshown -= len(page.entries)
	}
	cards := make([]*ChatCard, 0, len(pages))
	for i, page := range pages {
		suffix := ""
		if len(pages) > 1 {
			suffix = fmt.Sprintf(" · %d/%d", i+1, len(pages))
		}
		if i == len(pages)-1 && unshown > 0 {
			suffix += fmt.Sprintf(" · 另有 %d 条未展示", unshown)
		}
		card := renderPage(page.entries, page.nodes, suffix)
		if !fitsCard(card) {
			return nil, fmt.Errorf("RMTV card %d exceeds send limits", i+1)
		}
		cards = append(cards, card)
	}
	return cards, nil
}
