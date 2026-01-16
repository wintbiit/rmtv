package main

import (
	"context"
	"os"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gorilla/feeds"
	"github.com/joho/godotenv"
	"github.com/samber/lo"
	"github.com/sirupsen/logrus"
	"github.com/wintbiit/rmtv/ent"
	"github.com/wintbiit/rmtv/ent/post"

	_ "github.com/lib/pq"
)

func main() {
	godotenv.Load()
	dbUrl, ok := os.LookupEnv("DB_URL")
	if !ok {
		panic("DB_URL is required")
	}

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}

	maxFeedItemCount := os.Getenv("MAX_FEED_ITEM_COUNT")
	if maxFeedItemCount == "" {
		maxFeedItemCount = "10"
	}
	maxCount, err := strconv.Atoi(maxFeedItemCount)
	if err != nil {
		panic(err)
	}

	db, err := ent.Open("postgres", dbUrl)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	if err := db.Schema.Create(context.Background()); err != nil {
		panic(err)
	}

	app := fiber.New()
	app.Use(recover.New())
	app.Use(logger.New())

	getFeeds := func(c *fiber.Ctx) error {
		source := c.Params("source")

		query := db.Post.Query().
			Order(ent.Desc(post.FieldCreatedAt)).
			Limit(maxCount)

		if source != "" {
			query = query.Where(post.SourceEQ(source))
		}

		posts, err := query.All(c.Context())
		if err != nil {
			logrus.Errorf("failed to query posts: %v", err)
			return fiber.ErrInternalServerError
		}

		items := lo.Map(posts, func(item *ent.Post, _ int) *feeds.Item {
			return &feeds.Item{
				Title: item.Title,
				Link: &feeds.Link{
					Href: item.URL,
				},
				Source: &feeds.Link{
					Href: item.Source,
				},
				Author: &feeds.Author{
					Name: item.Author,
				},
				Description: item.Description,
				Id:          item.ID,
				Updated:     item.UpdatedAt,
				Created:     item.CreatedAt,
				Enclosure:   nil,
			}
		})

		c.Locals("feeds", items)

		return nil
	}

	app.Get("/rss", getFeeds)
	app.Get("/rss/:source", getFeeds)

	app.Use(func(c *fiber.Ctx) error {
		responseType := c.Query("type")
		if responseType == "" {
			responseType = "rss"
		}

		if !lo.Contains([]string{"rss", "atom", "json"}, responseType) {
			return fiber.ErrBadRequest
		}

		if err := c.Next(); err != nil {
			return err
		}

		feed := &feeds.Feed{
			Title:       "rmtv",
			Link:        &feeds.Link{Href: "https://github.com/wintbiit/rmtv"},
			Description: "rmtv feeds " + c.Params("source"),
			Created:     time.Now(),
			Items:       c.Locals("feeds").([]*feeds.Item),
		}

		switch responseType {
		case "rss":
			rss, err := feed.ToRss()
			if err != nil {
				return err
			}

			c.Set("Content-Type", "application/rss+xml")
			return c.SendString(rss)
		case "atom":
			atom, err := feed.ToAtom()
			if err != nil {
				return err
			}

			c.Set("Content-Type", "application/atom+xml")
			return c.SendString(atom)
		case "json":
			json, err := feed.ToJSON()
			if err != nil {
				return err
			}

			c.Set("Content-Type", "application/json")
			return c.SendString(json)
		}

		return nil
	})

	if err := app.Listen(addr); err != nil {
		panic(err)
	}
}
