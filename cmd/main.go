package main

import (
	"context"
	botcommands "d0c/articlesCollectorBot/internal/bot_commands"
	"d0c/articlesCollectorBot/internal/bot_commands/middleware"
	"d0c/articlesCollectorBot/internal/botkit"
	"d0c/articlesCollectorBot/internal/config"
	"d0c/articlesCollectorBot/internal/fetcher"
	"d0c/articlesCollectorBot/internal/notifier"
	"d0c/articlesCollectorBot/internal/storage"
	"d0c/articlesCollectorBot/internal/summary"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func main() {
	botAPI, err := tgbotapi.NewBotAPI(config.Get().TelegramBotToken)
	if err != nil {
		log.Printf("[ERROR] failed to create bot: %v", err) //Не применяю log.Fatal, ибо в таком случае defer в функциях не сработают
		return
	}

	db, err := sqlx.Connect("postgres", config.Get().DatabaseDSN)
	if err != nil {
		log.Printf("[ERROR] Failed to connect to database: %v", err)
		return
	}
	defer db.Close()

	var (
		articleStorage = storage.NewArticleStorage(db)
		sourceStorage  = storage.NewSourceStorage(db)
		fetcher        = fetcher.New(
			articleStorage,
			sourceStorage,
			config.Get().FetchInterval,
			config.Get().FilterKeywords,
		)
		summarizer = summary.NewOpenAISummarizer(
			config.Get().OpenAIKey,
			config.Get().OpenAIModel,
			config.Get().OpenAIPrompt,
		)
		notifier = notifier.New(
			articleStorage,
			summarizer,
			botAPI,
			config.Get().NotificationInterval,
			2*config.Get().FetchInterval,
			config.Get().TelegramChannelID,
		)
	)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	newsBot := botkit.New(botAPI)
	newsBot.RegisterCmdView("start", botcommands.ViewCmdStart())
	newsBot.RegisterCmdView(
		"addsource",
		middleware.AdminOnly(
			config.Get().TelegramChannelID,
			botcommands.ViewCmdAddSource(sourceStorage),
		),
	)
	newsBot.RegisterCmdView(
		"listsource",
		middleware.AdminOnly(
			config.Get().TelegramChannelID,
			botcommands.ViewCmdListSources(sourceStorage),
		),
	)

	go func(ctx context.Context) {
		if err := fetcher.Start(ctx); err != nil {
			if !errors.Is(err, context.Canceled) {
				log.Printf("[ERROR] failed to start fetcher: %v", err)
				return
			}

			log.Println("Fetcher stopped")
		}
	}(ctx)

	go func(ctx context.Context) {
		if err := notifier.Start(ctx); err != nil {
			if !errors.Is(err, context.Canceled) {
				log.Printf("[ERROR] failed to start notifier: %v", err)
				return
			}

			log.Println("Notifier stopped")
		}
	}(ctx)

	if err := newsBot.Run(ctx); err != nil {
		log.Printf("[ERROR] Failed to run bot: %v", err)
	}
}
