package handlers

import (
	"context"

	maxbot "github.com/max-messenger/max-bot-api-client-go"
	"github.com/max-messenger/max-bot-api-client-go/schemes"

	"kontroler-ts/state"
)

func (b *Bot) showMainMenu(ctx context.Context, chatID int64) {
	state.SetState(chatID, state.StateMainMenu)
	b.sendWithKeyboard(ctx, chatID, "Главное меню:", func(kb *maxbot.Keyboard) {
		kb.AddRow().AddCallback("👤 Мой профиль", schemes.DEFAULT, "menu:profile")
		kb.AddRow().AddCallback("📝 Заполнить журнал обходов (осмотров)", schemes.DEFAULT, "menu:journal")
		kb.AddRow().AddCallback("🔧 Мероприятия по результатам обходов", schemes.DEFAULT, "menu:measures")
		kb.AddRow().AddCallback("🧠 Помощь экспертов и предложения", schemes.DEFAULT, "menu:experts")
	})
}
