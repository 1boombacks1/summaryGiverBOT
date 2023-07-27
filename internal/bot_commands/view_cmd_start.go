package botcommands

import (
	"context"
	"d0c/articlesCollectorBot/internal/botkit"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func ViewCmdStart() botkit.ViewFunc {
	return func(ctx context.Context, bot *tgbotapi.BotAPI, update tgbotapi.Update) error {
		if _, err := bot.Send(tgbotapi.NewMessage(update.FromChat().ID, "Приветсвую, Приятель! Рад видеть тебя в здравии!")); err != nil {
			return err
		}

		return nil
	}
}
