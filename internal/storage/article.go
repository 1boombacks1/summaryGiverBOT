package storage

import (
	"context"
	"d0c/articlesCollectorBot/internal/model"
	"database/sql"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/samber/lo"
)

type ArticlePostgresStorage struct {
	db *sqlx.DB
}

func NewArticleStorage(db *sqlx.DB) *ArticlePostgresStorage {
	return &ArticlePostgresStorage{
		db: db,
	}
}

//Метод сохранения статьи в базу данных
func (s *ArticlePostgresStorage) Store(ctx context.Context, article model.Article) error {
	conn, err := s.db.Connx(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	if _, err := conn.ExecContext(
		ctx,
		`INSERT INTO articles (source_id, title, link, summary, published_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT DO NOTHING`,
		article.SourceID,
		article.Title,
		article.Link,
		article.Summary,
		article.PublishedAt,
	); err != nil {
		return err
	}

	return nil
}

//Возвращает все статьи, которые не были опубликованы в тг канал
func (s *ArticlePostgresStorage) AllNotPosted(ctx context.Context, limit uint64) ([]model.Article, error) {
	conn, err := s.db.Connx(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	var articles []dbArticle

	if err := conn.SelectContext(
		ctx,
		&articles,
		`SELECT * FROM articles
		WHERE posted_at IS NULL
		ORDER BY published_at DESC
		LIMIT $1`, //введя ::timestamp мы говорим Postgres, что тут timestamp
		//postgres понимает время в таком формате, из-за чего он должен все правильно распарсить
		limit,
	); err != nil {
		return nil, err
	}

	return lo.Map(articles, func(article dbArticle, _ int) model.Article {
		return model.Article{
			ID:          article.ID,
			SourceID:    article.SourceID,
			Title:       article.Title,
			Link:        article.Link,
			Summary:     article.Summary,
			PostedAt:    article.PostedAt.Time,
			PublishedAt: article.PublishedAt,
		}
	}), nil
}

//Отмечает статьи, как опубликованную
func (s *ArticlePostgresStorage) MarkPosted(ctx context.Context, id int64) error {
	conn, err := s.db.Connx(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	if _, err := conn.ExecContext(
		ctx,
		`UPDATE articles SET posted_at = $1::timestamp WHERE id = $2`,
		time.Now().UTC().Format(time.RFC3339),
		id,
	); err != nil {
		return err
	}

	return nil
}

//Внутренний тип для работы с бд
type dbArticle struct {
	ID          int64        `db:"id"`
	SourceID    int64        `db:"source_id"`
	Title       string       `db:"title"`
	Link        string       `db:"link"`
	Summary     string       `db:"summary"`
	CreatedAt   time.Time    `db:"created_at"`
	PostedAt    sql.NullTime `db:"posted_at"`
	PublishedAt time.Time    `db:"published_at"`
}
