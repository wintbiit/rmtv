package job

import "time"

// Item is the content shared by all card layouts.
type Item struct {
	ID, Source, Number, Title, URL, Author, Body string
	PublishedAt                                  time.Time
	Images                                       []ItemImage
}

type ItemImage struct {
	URL, Caption string
	AfterTitle   bool
}

func ItemFromPost(post Post) Item {
	item := Item{
		ID: post.GetId(), Source: post.GetType(), Title: post.GetTitle(),
		URL: post.GetUrl(), Author: post.GetAuthor(), Body: post.GetDesc(),
		PublishedAt: post.GetPubDate(),
	}
	if details, ok := post.(interface {
		CardNumber() string
		CardImages() []ItemImage
	}); ok {
		item.Number = details.CardNumber()
		item.Images = details.CardImages()
	} else if picture := post.GetPic(); picture != nil && *picture != "" {
		item.Images = []ItemImage{{URL: *picture}}
	}
	return item
}
