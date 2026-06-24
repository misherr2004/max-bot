package handlers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	maxbot "github.com/max-messenger/max-bot-api-client-go"
	"github.com/max-messenger/max-bot-api-client-go/schemes"

	"kontroler-ts/reports"
	"kontroler-ts/state"
)

func (b *Bot) showAdminMenu(ctx context.Context, chatID int64) {
	state.SetState(chatID, state.StateAdminMenu)
	b.sendWithKeyboard(ctx, chatID, "Меню администратора:", func(kb *maxbot.Keyboard) {
		kb.AddRow().AddCallback("📤 Загрузить списки", schemes.DEFAULT, "admin:upload")
		kb.AddRow().AddCallback("👥 Управление экспертами", schemes.DEFAULT, "admin:experts")
		kb.AddRow().AddCallback("📊 Отчёт по пользователям", schemes.DEFAULT, "admin:report_users")
		kb.AddRow().AddCallback("📋 Отчёт по мероприятиям", schemes.DEFAULT, "admin:report_measures")
	})
}

func (b *Bot) handleAdminCallback(ctx context.Context, chatID, userID int64, payload string) {
	u, err := b.getUser(chatID, userID)
	if err != nil || u.Role != "admin" {
		b.sendText(ctx, chatID, "Доступ запрещён")
		return
	}

	switch payload {
	case "admin:upload":
		state.SetState(chatID, state.StateAdminUpload)
		b.sendText(ctx, chatID, `Отправьте Excel-файл (.xlsx) со следующими листами:
- Лист 'Эксперты': колонки [ФИО, Телефон]
- Лист 'Сотрудники': колонка [Телефон]
- Лист 'Адреса ТК': колонка [Адрес]`)
	case "admin:experts":
		b.showAdminExperts(ctx, chatID)
	case "admin:report_users":
		b.generateUsersReport(ctx, chatID)
	case "admin:report_measures":
		b.generateMeasuresReport(ctx, chatID)
	case "admin:expert_add":
		count, _ := b.DB.CountActiveExperts()
		if count >= 3 {
			b.sendText(ctx, chatID, "Максимум 3 эксперта")
			return
		}
		state.SetState(chatID, state.StateAdminAddExpert)
		b.sendText(ctx, chatID, "Введите номер телефона эксперта:")
	case "admin:expert_remove":
		b.showAdminRemoveExperts(ctx, chatID)
	}
}

func (b *Bot) showAdminExperts(ctx context.Context, chatID int64) {
	experts, _ := b.DB.GetActiveExperts()
	text := "Текущие эксперты:\n"
	for _, e := range experts {
		text += fmt.Sprintf("• %s (%s)\n", e.User.FullName(), e.User.Phone)
	}
	if len(experts) == 0 {
		text = "Эксперты не назначены.\n"
	}
	b.sendWithKeyboard(ctx, chatID, text, func(kb *maxbot.Keyboard) {
		kb.AddRow().AddCallback("➕ Добавить", schemes.DEFAULT, "admin:expert_add")
		kb.AddRow().AddCallback("🗑 Удалить", schemes.DEFAULT, "admin:expert_remove")
		kb.AddRow().AddCallback("🔙 Назад", schemes.DEFAULT, "admin:back")
	})
}

func (b *Bot) showAdminRemoveExperts(ctx context.Context, chatID int64) {
	experts, _ := b.DB.GetActiveExperts()
	state.SetState(chatID, state.StateAdminRemoveExpert)
	b.sendWithKeyboard(ctx, chatID, "Выберите эксперта для удаления:", func(kb *maxbot.Keyboard) {
		for _, e := range experts {
			kb.AddRow().AddCallback(e.User.FullName(), schemes.DEFAULT, fmt.Sprintf("admin:rm_expert:%d", e.ID))
		}
	})
}

func (b *Bot) handleAdminAddExpertPhone(ctx context.Context, chatID int64, phone string) {
	phone = normalizePhone(phone)
	if !phoneRe().MatchString(phone) {
		b.sendText(ctx, chatID, "Введите номер в формате +7XXXXXXXXXX")
		return
	}
	count, _ := b.DB.CountActiveExperts()
	if count >= 3 {
		b.sendText(ctx, chatID, "Максимум 3 эксперта")
		return
	}
	if err := b.DB.UpsertExpertByPhone(phone); err != nil {
		b.sendText(ctx, chatID, "Ошибка: "+err.Error())
		return
	}
	b.sendText(ctx, chatID, "✅ Эксперт добавлен")
	state.SetState(chatID, state.StateAdminMenu)
	b.showAdminMenu(ctx, chatID)
}

func (b *Bot) tryHandleFile(ctx context.Context, chatID int64, upd *schemes.MessageCreatedUpdate) bool {
	if state.GetCurrentState(chatID) != state.StateAdminUpload {
		return false
	}
	for _, att := range upd.Message.Body.Attachments {
		f, ok := att.(*schemes.FileAttachment)
		if !ok {
			continue
		}
		path, err := b.downloadAttachment(ctx, f.Payload.Url, f.Filename)
		if err != nil {
			b.sendText(ctx, chatID, "Ошибка загрузки файла: "+err.Error())
			return true
		}
		if err := reports.ImportLists(b.DB, path); err != nil {
			b.sendText(ctx, chatID, "Ошибка разбора Excel: "+err.Error())
			return true
		}
		b.sendText(ctx, chatID, "✅ Списки обновлены")
		state.SetState(chatID, state.StateAdminMenu)
		b.showAdminMenu(ctx, chatID)
		return true
	}
	return false
}

func (b *Bot) downloadAttachment(ctx context.Context, url, filename string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	ensureUploadsDir(b.UploadsDir)
	if filename == "" {
		filename = "upload.xlsx"
	}
	path := filepath.Join(b.UploadsDir, filename)
	out, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer out.Close()
	_, err = io.Copy(out, resp.Body)
	return path, err
}

func (b *Bot) generateUsersReport(ctx context.Context, chatID int64) {
	users, err := b.DB.GetAllUsers()
	if err != nil {
		b.sendText(ctx, chatID, "Ошибка формирования отчёта")
		return
	}
	path := filepath.Join(b.UploadsDir, "users_report.xlsx")
	ensureUploadsDir(b.UploadsDir)
	if err := reports.ExportUsers(users, path); err != nil {
		b.sendText(ctx, chatID, "Ошибка: "+err.Error())
		return
	}
	_ = b.sendFile(ctx, chatID, path, "Отчёт по пользователям:")
}

func (b *Bot) generateMeasuresReport(ctx context.Context, chatID int64) {
	items, err := b.DB.GetAllMeasuresWithDetails()
	if err != nil {
		b.sendText(ctx, chatID, "Ошибка формирования отчёта")
		return
	}
	path := filepath.Join(b.UploadsDir, "measures_report.xlsx")
	ensureUploadsDir(b.UploadsDir)
	if err := reports.ExportMeasures(items, path); err != nil {
		b.sendText(ctx, chatID, "Ошибка: "+err.Error())
		return
	}
	_ = b.sendFile(ctx, chatID, path, "Отчёт по мероприятиям:")
}

// handle admin remove expert callback from bot.go
func (b *Bot) handleAdminRemoveExpert(ctx context.Context, chatID int64, payload string) {
	idStr := strings.TrimPrefix(payload, "admin:rm_expert:")
	id, _ := strconv.ParseInt(idStr, 10, 64)
	_ = b.DB.RemoveExpert(id)
	b.sendText(ctx, chatID, "✅ Эксперт удалён")
	b.showAdminExperts(ctx, chatID)
}
