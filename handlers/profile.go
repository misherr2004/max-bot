package handlers

import (
	"context"
	"fmt"

	maxbot "github.com/max-messenger/max-bot-api-client-go"
	"github.com/max-messenger/max-bot-api-client-go/schemes"

	"kontroler-ts/state"
)

func (b *Bot) showProfile(ctx context.Context, chatID, userID int64) {
	u, err := b.getUser(chatID, userID)
	if err != nil {
		b.sendText(ctx, chatID, "Профиль не найден. Нажмите /start")
		return
	}
	journals, _ := b.DB.CountJournals(u.ID)
	measures, _ := b.DB.CountMeasures(u.ID)
	lastLogin := "—"
	if u.LastLogin != nil {
		lastLogin = formatDate(*u.LastLogin)
	}
	text := fmt.Sprintf(`👤 %s
📋 Должность: %s
⏱ Стаж: %s
🏭 Участок: %s
📱 Телефон: %s

📊 Статистика:
• Журналов обходов: %d шт.
• Мероприятий: %d шт.
• Последний вход: %s`,
		u.FullName(), u.Position, u.Experience, u.WorkArea, u.Phone,
		journals, measures, lastLogin)

	state.SetState(chatID, state.StateProfile)
	b.sendWithKeyboard(ctx, chatID, text, func(kb *maxbot.Keyboard) {
		kb.AddRow().AddCallback("✏️ Редактировать данные", schemes.DEFAULT, "profile:edit")
		kb.AddRow().AddCallback("🔙 Назад", schemes.DEFAULT, "menu:back")
	})
}

func (b *Bot) handleProfileEdit(ctx context.Context, chatID, userID int64, workArea string) {
	u, err := b.getUser(chatID, userID)
	if err != nil {
		return
	}
	_ = b.DB.UpdateUserProfile(u.ID, u.FirstName, u.Patronymic, u.LastName, u.Position, u.Experience, workArea)
	state.SetState(chatID, state.StateMainMenu)
	b.sendText(ctx, chatID, "✅ Данные обновлены")
	b.showMainMenu(ctx, chatID)
}
