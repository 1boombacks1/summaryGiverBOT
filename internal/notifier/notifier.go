package notifier

import (
	"context"
	"d0c/articlesCollectorBot/internal/botkit/markup"
	"d0c/articlesCollectorBot/internal/model"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-shiori/go-readability"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type ArticleProvider interface {
	AllNotPosted(ctx context.Context, limit uint64) ([]model.Article, error)
	MarkPosted(ctx context.Context, id int64) error
}

type Summarizer interface {
	Summarize(text string) (string, error)
}

type Notifier struct {
	articles         ArticleProvider
	summarizer       Summarizer       //Компонент, который генерирует краткое содержание статьи
	bot              *tgbotapi.BotAPI //Клиент tgBotAPI
	sendInterval     time.Duration    //По данному интервалу Notifier будет проверять источникик на наличия новых статей
	lookupTimeWindow time.Duration    //Время, с которого Notifier будет проверять есть ли новые статьи
	channelID        int64            //id канала куда будут выкладываться статьи
}

func New(
	articleProvider ArticleProvider,
	summarizer Summarizer,
	bot *tgbotapi.BotAPI,
	sendInterval time.Duration,
	lookupTimeWindow time.Duration,
	channelID int64,
) *Notifier {
	return &Notifier{
		articles:         articleProvider,
		summarizer:       summarizer,
		bot:              bot,
		sendInterval:     sendInterval,
		lookupTimeWindow: lookupTimeWindow,
		channelID:        channelID,
	}
}

func (n *Notifier) Start(ctx context.Context) error {
	ticker := time.NewTicker(n.sendInterval)
	defer ticker.Stop()

	if err := n.SelectAndSendArticle(ctx); err != nil {
		return err
	}

	for {
		select {
		case <-ticker.C:
			if err := n.SelectAndSendArticle(ctx); err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

/*TODO:Тут следует обернуть все действия в транзакцую, так как если на каком-то этапе
произойдет непредвиденная ошибка, все что было сделано ДО - не откатиться, что
выльется в проблемы: 1. Улетят деньги за написания саммари 2. Статья опубликуется дважды
*/
func (n *Notifier) SelectAndSendArticle(ctx context.Context) error {
	topOneArticle, err := n.articles.AllNotPosted(ctx, 1)
	log.Println("[LOG] Получение неопубликованных статей")
	if err != nil {
		return err
	}

	if len(topOneArticle) == 0 {
		log.Println("[LOG] There are no unpublished articles")
		return nil
	}

	article := topOneArticle[0]

	log.Println("[LOG] Подготовка к саммери...")
	summary, err := n.extractSummary(article)
	if err != nil {
		log.Printf("[ERROR] failed to extract summary: %v", err)
	}

	log.Println("[LOG] Подготовка к отправке...")
	if err := n.sendArticle(article, summary); err != nil {
		return err
	}

	log.Printf("[LOG] The article '%s' has been sent", article.Title)

	return n.articles.MarkPosted(ctx, article.ID)
}

func (n *Notifier) sendArticle(article model.Article, summary string) error {
	const msgFormat = "*%s*%s\n\n%s"

	msg := tgbotapi.NewMessage(n.channelID, fmt.Sprintf(
		msgFormat,
		markup.EscapeForMarkdown(article.Title),
		markup.EscapeForMarkdown(summary),
		markup.EscapeForMarkdown(article.Link),
	))
	msg.ParseMode = tgbotapi.ModeMarkdownV2

	_, err := n.bot.Send(msg)
	if err != nil {
		return err
	}

	return nil
}

func (n *Notifier) extractSummary(article model.Article) (string, error) {
	var r io.Reader

	if article.Summary != "" {
		r = strings.NewReader(article.Summary)
	} else {
		resp, err := http.Get(article.Link)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()

		r = resp.Body
	}

	doc, err := readability.FromReader(r, nil)
	if err != nil {
		return "", err
	}

	summary, err := n.summarizer.Summarize(cleanText(doc.TextContent))
	if err != nil {
		return "", err
	}

	return "\n\n" + summary, nil
}

var redundantNewLines = regexp.MustCompile(`\n{3,}`)

func cleanText(text string) string {
	return redundantNewLines.ReplaceAllString(text, "\n")
}
