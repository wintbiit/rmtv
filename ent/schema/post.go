package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Post holds the schema definition for the Post entity.
type Post struct {
	ent.Schema
}

// Fields of the Post.
func (Post) Fields() []ent.Field {
	return []ent.Field{
		field.String("source").NotEmpty().Comment("来源"),
		field.String("id").NotEmpty().Comment("ID"),
		field.String("picture").Optional().Nillable().Comment("图片"),
		field.String("title").NotEmpty().Comment("标题"),
		field.String("description").Comment("描述"),
		field.Strings("tags").Comment("标签"),
		field.Time("pub_date").Comment("发布时间"),
		field.String("author").Comment("作者"),
		field.String("author_url").Comment("作者链接"),
		field.String("url").Comment("链接"),
		field.Any("extra").Comment("额外信息"),
		field.Time("created_at").Default(time.Now).Comment("创建时间"),
		field.Time("updated_at").Default(time.Now).Comment("更新时间"),
	}
}

// Edges of the Post.
func (Post) Edges() []ent.Edge {
	return nil
}

func (Post) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("source", "id").Unique(),
	}
}
