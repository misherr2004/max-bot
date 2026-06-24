package handlers

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	maxbot "github.com/max-messenger/max-bot-api-client-go"
	"github.com/max-messenger/max-bot-api-client-go/schemes"

	"kontroler-ts/db"
	"kontroler-ts/state"
)

func (b *Bot) showMeasures(ctx context.Context, chatID, userID int64) {
	u, err := b.getUser(chatID, userID)
	if err != nil {
		return
	}
	j, err := b.DB.GetLastJournal(u.ID)
	if err != nil || j == nil {
		b.sendText(ctx, chatID, "Сначала заполните и сохраните журнал обходов.")
		return
	}

	issues := db.CollectJournalIssues(j)
	comments := db.CollectJournalComments(j)
	issuesText := "—"
	if len(issues) > 0 {
		issuesText = strings.Join(issues, "\n• ")
		if !strings.HasPrefix(issuesText, "•") {
			issuesText = "• " + issuesText
		}
	}
	commentsText := comments
	if commentsText == "" {
		commentsText = "—"
	}

	text := fmt.Sprintf(`📋 Мероприятия по обходу %s
📍 %s

⚠️ Выявленные замечания:
%s

💬 Комментарии:
%s`, j.InspectionDate, j.TkAddress, issuesText, commentsText)

	state.SetState(chatID, state.StateMeasures)
	state.SetData(chatID, "measure_journal_id", j.ID)
	b.sendWithKeyboard(ctx, chatID, text, func(kb *maxbot.Keyboard) {
		kb.AddRow().AddCallback("💾 Сохранить мероприятия", schemes.DEFAULT, "measures:save")
		kb.AddRow().AddCallback("✏️ Редактировать", schemes.DEFAULT, "measures:edit")
		kb.AddRow().AddCallback("❌ Отмена", schemes.DEFAULT, "measures:cancel")
	})
}

func (b *Bot) saveMeasures(ctx context.Context, chatID, userID int64) {
	u, err := b.getUser(chatID, userID)
	if err != nil {
		return
	}
	journalID := getInt64(chatID, "measure_journal_id")
	if journalID == 0 {
		j, _ := b.DB.GetLastJournal(u.ID)
		if j != nil {
			journalID = j.ID
		}
	}
	if journalID == 0 {
		b.sendText(ctx, chatID, "Журнал не найден")
		return
	}

	_, err = b.DB.CreateMeasure(journalID, u.ID)
	if err != nil {
		b.sendText(ctx, chatID, "Ошибка сохранения мероприятий")
		return
	}

	j, _ := b.DB.GetLastJournal(u.ID)
	issues := db.CollectJournalIssues(j)
	shortIssues := strings.Join(issues, "; ")
	if len(shortIssues) > 200 {
		shortIssues = shortIssues[:200] + "..."
	}

	experts, _ := b.DB.GetActiveExperts()
	for _, e := range experts {
		expertChat, _ := strconv.ParseInt(e.User.MaxUserID, 10, 64)
		b.sendWithKeyboard(ctx, expertChat,
			fmt.Sprintf("🔔 Новое мероприятие от %s\nДата: %s. Адрес: %s\n%s",
				u.ShortName(), j.InspectionDate, j.TkAddress, shortIssues),
			func(kb *maxbot.Keyboard) {
				kb.AddRow().AddCallback("👁 Посмотреть", schemes.DEFAULT, fmt.Sprintf("measure_view:%d", journalID))
			})
	}

	b.sendText(ctx, chatID, fmt.Sprintf("🎉 %s, Вы отлично поработали!", u.Patronymic))
	state.SetState(chatID, state.StateMainMenu)
	b.showMainMenu(ctx, chatID)
}
