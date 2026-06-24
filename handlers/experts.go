package handlers

import (
	"context"
	"fmt"
	"strconv"

	maxbot "github.com/max-messenger/max-bot-api-client-go"
	"github.com/max-messenger/max-bot-api-client-go/schemes"

	"kontroler-ts/db"
	"kontroler-ts/state"
)

func (b *Bot) showExperts(ctx context.Context, chatID int64) {
	experts, err := b.DB.GetActiveExperts()
	if err != nil || len(experts) == 0 {
		b.sendWithKeyboard(ctx, chatID, "👥 Список экспертов пуст.", func(kb *maxbot.Keyboard) {
			kb.AddRow().AddCallback("🔙 Главное меню", schemes.DEFAULT, "menu:back")
		})
		return
	}

	state.SetState(chatID, state.StateExperts)
	b.sendWithKeyboard(ctx, chatID, "👥 Список экспертов:", func(kb *maxbot.Keyboard) {
		for _, e := range experts {
			label := fmt.Sprintf("%s | стаж: %s", e.User.FullName(), e.User.Experience)
			kb.AddRow().AddCallback(label, schemes.DEFAULT, fmt.Sprintf("expert_chat:%d", e.User.ID))
		}
		kb.AddRow().AddCallback("🔙 Главное меню", schemes.DEFAULT, "menu:back")
	})
}

func (b *Bot) startExpertChat(ctx context.Context, chatID int64, expertUserIDStr string) {
	expertID, _ := strconv.ParseInt(expertUserIDStr, 10, 64)
	expert, err := b.DB.GetUserByID(expertID)
	if err != nil {
		return
	}
	state.SetData(chatID, "expert_to_id", expertID)
	state.SetState(chatID, state.StateExpertChatMsg)
	b.sendText(ctx, chatID, fmt.Sprintf("Пользователю %s будет отправлено приглашение на общение\n\nВведите ваше сообщение:", expert.ShortName()))
}

func (b *Bot) handleExpertMessage(ctx context.Context, chatID, userID int64, text string) {
	u, err := b.getUser(chatID, userID)
	if err != nil {
		return
	}
	expertID := getInt64(chatID, "expert_to_id")
	expert, err := b.DB.GetUserByID(expertID)
	if err != nil {
		return
	}

	_ = b.DB.SaveMessage(u.ID, expert.ID, text)

	expertChat, _ := strconv.ParseInt(expert.MaxUserID, 10, 64)
	msg := fmt.Sprintf("💬 Пользователь %s пригласил вас на общение\n\n%s", u.ShortName(), text)

	j, _ := b.DB.GetLastJournal(u.ID)
	if j != nil && j.Status == "saved" {
		issues := db.CollectJournalIssues(j)
		if len(issues) > 0 {
			msg += "\n\nПоследнее мероприятие:\n" + issues[0]
		}
	}

	b.sendWithKeyboard(ctx, expertChat, msg, func(kb *maxbot.Keyboard) {
		kb.AddRow().AddCallback("↩️ Ответить", schemes.DEFAULT, fmt.Sprintf("reply:%d", u.ID))
	})

	b.sendWithKeyboard(ctx, chatID, "✅ Сообщение отправлено", func(kb *maxbot.Keyboard) {
		kb.AddRow().AddCallback("🔙 Главное меню", schemes.DEFAULT, "menu:back")
		kb.AddRow().AddCallback("📋 К списку экспертов", schemes.DEFAULT, "experts:list")
	})
	state.SetState(chatID, state.StateExperts)
}

func (b *Bot) handleExpertReply(ctx context.Context, chatID, expertUserID int64, payload string) {
	fromIDStr := stringsTrimPrefix(payload, "reply:")
	fromID, _ := strconv.ParseInt(fromIDStr, 10, 64)
	state.SetData(chatID, "expert_to_id", fromID)
	state.SetState(chatID, state.StateExpertChatMsg)
	b.sendText(ctx, chatID, "Введите ваш ответ:")
}

func stringsTrimPrefix(s, prefix string) string {
	if len(s) >= len(prefix) && s[:len(prefix)] == prefix {
		return s[len(prefix):]
	}
	return s
}
