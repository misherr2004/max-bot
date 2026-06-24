package handlers

import (
	"context"
	"fmt"
	"strconv"
	"time"

	maxbot "github.com/max-messenger/max-bot-api-client-go"
	"github.com/max-messenger/max-bot-api-client-go/schemes"

	"kontroler-ts/state"
)

func (b *Bot) handleStart(ctx context.Context, chatID, userID int64) {
	state.ClearSession(chatID)
	state.SetState(chatID, state.StateStart)
	state.SetData(chatID, "max_user_id", strconv.FormatInt(userID, 10))

	text := "Добро пожаловать в наше подземелье! 🏰\nЧат бот «Контролер-ТС» - оформляю результаты осмотра оборудования."
	b.sendWithKeyboard(ctx, chatID, text, func(kb *maxbot.Keyboard) {
		kb.AddRow().AddCallback("🚀 Начать осмотр", schemes.DEFAULT, "action:start")
		kb.AddRow().AddCallback("👋 Выход", schemes.DEFAULT, "action:exit")
	})
}

func (b *Bot) askPhone(ctx context.Context, chatID int64) {
	state.SetState(chatID, state.StateWaitPhone)
	text := "Прошу предоставить номер телефона для проверки Пользователя"
	b.sendWithKeyboard(ctx, chatID, text, func(kb *maxbot.Keyboard) {
		kb.AddRow().AddCallback("📱 Поделиться номером", schemes.DEFAULT, "action:share_phone")
		kb.AddRow().AddCallback("👋 Выход", schemes.DEFAULT, "action:exit")
	})
}

func (b *Bot) handlePhoneInput(ctx context.Context, chatID, userID int64, raw string) {
	phone := normalizePhone(raw)
	if !phoneRe().MatchString(phone) {
		b.sendText(ctx, chatID, "Введите номер в формате +7XXXXXXXXXX")
		return
	}

	state.SetData(chatID, "phone", phone)
	maxUserID := strconv.FormatInt(userID, 10)

	allowed, _ := b.DB.IsPhoneAllowed(phone)
	user, err := b.DB.GetUserByPhone(phone)

	if allowed && err == nil {
		_ = b.DB.UpdateLastLogin(user.ID)
		if user.Role == "admin" {
			state.SetState(chatID, state.StateAdminMenu)
			b.showAdminMenu(ctx, chatID)
			return
		}
		state.SetState(chatID, state.StateMainMenu)
		b.showMainMenu(ctx, chatID)
		return
	}

	if !allowed {
		b.requestPhoneApproval(ctx, chatID, phone, maxUserID)
		return
	}

	// allowed but not registered
	_, _ = b.DB.GetUserByMaxID(maxUserID)
	if user == nil {
		_, _ = b.DB.CreateUser(maxUserID, phone)
	}
	b.startRegistration(ctx, chatID)
}

func (b *Bot) requestPhoneApproval(ctx context.Context, chatID int64, phone, maxUserID string) {
	state.SetState(chatID, state.StateWaitPhoneApproval)
	state.SetData(chatID, "phone", phone)
	b.sendText(ctx, chatID, "Ваши данные отправлены на проверку Администратору ⏳")

	admins, _ := b.DB.GetAdmins()
	for _, admin := range admins {
		adminChat, _ := strconv.ParseInt(admin.MaxUserID, 10, 64)
		b.sendWithKeyboard(ctx, adminChat,
			fmt.Sprintf("📱 Новый пользователь хочет войти: %s. Добавить его?", phone),
			func(kb *maxbot.Keyboard) {
				kb.AddRow().
					AddCallback("✅ Добавить", schemes.DEFAULT, fmt.Sprintf("admin_approve:%d:%s", chatID, phone)).
					AddCallback("❌ Отклонить", schemes.DEFAULT, fmt.Sprintf("admin_reject:%d", chatID))
			})
	}

	approvalCh := make(chan bool, 1)
	b.pendingMu.Lock()
	b.pending[phone] = approvalCh
	b.pendingMu.Unlock()

	go func() {
		select {
		case approved := <-approvalCh:
			if approved {
				_ = b.DB.AddAllowedPhone(phone)
				_, err := b.DB.GetUserByMaxID(maxUserID)
				if err != nil {
					_, _ = b.DB.CreateUser(maxUserID, phone)
				}
				b.startRegistration(context.Background(), chatID)
			}
		case <-time.After(60 * time.Second):
			if state.GetCurrentState(chatID) == state.StateWaitPhoneApproval {
				state.ClearSession(chatID)
				b.sendText(context.Background(), chatID, "До новых встреч 👋")
			}
		}
		b.pendingMu.Lock()
		delete(b.pending, phone)
		b.pendingMu.Unlock()
	}()
}

func (b *Bot) handleAdminApprove(ctx context.Context, adminUserID int64, payload string) {
	parts := splitPayload(payload, 3)
	if len(parts) < 3 {
		return
	}
	chatID, _ := strconv.ParseInt(parts[1], 10, 64)
	phone := parts[2]

	admin, err := b.DB.GetUserByMaxID(strconv.FormatInt(adminUserID, 10))
	if err != nil || admin.Role != "admin" {
		return
	}

	_ = b.DB.AddAllowedPhone(phone)
	b.pendingMu.Lock()
	if ch, ok := b.pending[phone]; ok {
		ch <- true
	}
	b.pendingMu.Unlock()

	b.sendText(ctx, chatID, "✅ Ваш номер одобрен! Начинаем регистрацию.")
}

func splitPayload(s string, n int) []string {
	return stringsSplitN(s, ":", n)
}

// avoid importing strings in auth - use local helper
func stringsSplitN(s, sep string, n int) []string {
	var parts []string
	for i := 0; i < n-1; i++ {
		idx := indexOf(s, sep)
		if idx < 0 {
			parts = append(parts, s)
			return parts
		}
		parts = append(parts, s[:idx])
		s = s[idx+len(sep):]
	}
	parts = append(parts, s)
	return parts
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func (b *Bot) startRegistration(ctx context.Context, chatID int64) {
	state.SetState(chatID, state.StateRegName)
	b.sendText(ctx, chatID, "Введите ваше Имя:")
}

func (b *Bot) handleRegName(ctx context.Context, chatID int64, text string) {
	if !nameRe().MatchString(text) {
		b.sendText(ctx, chatID, "🤔 Не похоже на имя. Попробуйте еще раз")
		return
	}
	state.SetData(chatID, "first_name", text)
	state.SetState(chatID, state.StateRegPatronymic)
	b.sendText(ctx, chatID, "Введите ваше Отчество:")
}

func (b *Bot) handleRegPatronymic(ctx context.Context, chatID int64, text string) {
	if !nameRe().MatchString(text) {
		b.sendText(ctx, chatID, "🤔 Не похоже на отчество. Попробуйте еще раз")
		return
	}
	state.SetData(chatID, "patronymic", text)
	state.SetState(chatID, state.StateRegLastName)
	b.sendText(ctx, chatID, "Введите вашу Фамилию:")
}

func (b *Bot) handleRegLastName(ctx context.Context, chatID int64, text string) {
	if !nameRe().MatchString(text) {
		b.sendText(ctx, chatID, "🤔 Не похоже на фамилию. Попробуйте еще раз")
		return
	}
	state.SetData(chatID, "last_name", text)
	state.SetState(chatID, state.StateRegPosition)
	b.sendWithKeyboard(ctx, chatID, "Укажите должность:", func(kb *maxbot.Keyboard) {
		kb.AddRow().AddCallback("Слесарь", schemes.DEFAULT, "position:Слесарь")
		kb.AddRow().AddCallback("Мастер", schemes.DEFAULT, "position:Мастер")
		kb.AddRow().AddCallback("Старший мастер", schemes.DEFAULT, "position:Старший мастер")
		kb.AddRow().AddCallback("Начальник цеха/Района", schemes.DEFAULT, "position:Начальник цеха/Района")
	})
}

func (b *Bot) handleRegPosition(ctx context.Context, chatID int64, position string) {
	state.SetData(chatID, "position", position)
	state.SetState(chatID, state.StateRegExperience)
	b.sendWithKeyboard(ctx, chatID, "Укажите стаж:", func(kb *maxbot.Keyboard) {
		kb.AddRow().AddCallback("до 1 года", schemes.DEFAULT, "experience:до 1 года")
		kb.AddRow().AddCallback("от 1 до 3 лет", schemes.DEFAULT, "experience:от 1 до 3 лет")
		kb.AddRow().AddCallback("от 3 до 10 лет", schemes.DEFAULT, "experience:от 3 до 10 лет")
		kb.AddRow().AddCallback("более 10 лет", schemes.DEFAULT, "experience:более 10 лет")
	})
}

func (b *Bot) handleRegExperience(ctx context.Context, chatID, userID int64, exp string) {
	state.SetData(chatID, "experience", exp)
	state.SetState(chatID, state.StateRegWorkArea)
	b.sendText(ctx, chatID, "Введите участок работы:")
}

func (b *Bot) handleRegWorkArea(ctx context.Context, chatID, userID int64, workArea string) {
	maxUserID := strconv.FormatInt(userID, 10)
	phone := state.GetDataString(chatID, "phone")
	u, err := b.DB.GetUserByMaxID(maxUserID)
	if err != nil {
		u, _ = b.DB.CreateUser(maxUserID, phone)
	}
	_ = b.DB.UpdateUserProfile(u.ID,
		state.GetDataString(chatID, "first_name"),
		state.GetDataString(chatID, "patronymic"),
		state.GetDataString(chatID, "last_name"),
		state.GetDataString(chatID, "position"),
		state.GetDataString(chatID, "experience"),
		workArea,
	)
	state.SetState(chatID, state.StateMainMenu)
	b.showMainMenu(ctx, chatID)
}
