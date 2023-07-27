package botkit

import (
	"context"
	"log"
	"runtime/debug"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Bot struct {
	api      *tgbotapi.BotAPI
	cmdViews map[string]ViewFunc
}

//Это тип для функций, которые будут отрабатывать для какой-то конкретной команды
type ViewFunc func(ctx context.Context, bot *tgbotapi.BotAPI, update tgbotapi.Update) error

func New(api *tgbotapi.BotAPI) *Bot {
	return &Bot{
		api: api,
	}
}

func (bot *Bot) RegisterCmdView(cmd string, view ViewFunc) {
	if bot.cmdViews == nil {
		bot.cmdViews = make(map[string]ViewFunc)
	}

	bot.cmdViews[cmd] = view
}

func (bot *Bot) Run(ctx context.Context) error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.api.GetUpdatesChan(u)

	for {
		select {
		case update := <-updates:
			updateCtx, updateCancel := context.WithTimeout(context.Background(), 5*time.Minute)
			bot.handleUpdate(updateCtx, update)
			updateCancel()
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (bot *Bot) handleUpdate(ctx context.Context, update tgbotapi.Update) {
	//Тут мы перехватываем паники, и выводим в логах: панику и стак вызовов перед паникой(полезно для дебага)
	defer func() {
		if p := recover(); p != nil {
			log.Printf("[ERROR] panic recovered: %v\n%s", p, string(debug.Stack()))
		}
	}()

	if (update.Message == nil || !update.Message.IsCommand()) && update.CallbackQuery == nil {
		return
	}

	var view ViewFunc

	if !update.Message.IsCommand() {
		return
	}

	cmd := update.Message.Command()

	cmdView, ok := bot.cmdViews[cmd]
	if !ok {
		return
	}

	view = cmdView

	if err := view(ctx, bot.api, update); err != nil {
		log.Printf("[ERROR] Failed to handle update: %v", err)

		if _, err := bot.api.Send(
			tgbotapi.NewMessage(update.Message.Chat.ID, "internal error"),
		); err != nil {
			log.Printf("[ERROR] Failed to send message: %v", err)
		}
	}
}
