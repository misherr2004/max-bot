package handlers

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	maxbot "github.com/max-messenger/max-bot-api-client-go"
	"github.com/max-messenger/max-bot-api-client-go/schemes"

	"kontroler-ts/db"
	"kontroler-ts/state"
)

type Bot struct {
	API         *maxbot.Api
	DB          *db.DB
	UploadsDir  string
	pendingMu   sync.Mutex
	pending     map[string]chan bool // phone -> approval channel
}

func New(api *maxbot.Api, database *db.DB, uploadsDir string) *Bot {
	return &Bot{
		API:        api,
		DB:         database,
		UploadsDir: uploadsDir,
		pending:    make(map[string]chan bool),
	}
}

func (b *Bot) HandleUpdate(ctx context.Context, upd schemes.UpdateInterface) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[%s] PANIC recovered: %v", time.Now().Format(time.RFC3339), r)
		}
	}()

	log.Printf("[%s] Update: %T", time.Now().Format(time.RFC3339), upd)

	switch u := upd.(type) {
	case *schemes.BotStartedUpdate:
		b.handleStart(ctx, u.ChatId, u.User.UserId)
	case *schemes.MessageCreatedUpdate:
		b.handleMessage(ctx, u)
	case *schemes.MessageCallbackUpdate:
		b.handleCallback(ctx, u)
	}
}

func (b *Bot) handleMessage(ctx context.Context, upd *schemes.MessageCreatedUpdate) {
	chatID := upd.Message.Recipient.ChatId
	userID := upd.Message.Sender.UserId
	text := strings.TrimSpace(upd.Message.Body.Text)

	if text == "/start" {
		b.handleStart(ctx, chatID, userID)
		return
	}
	if text == "/menu" {
		u, err := b.DB.GetUserByMaxID(strconv.FormatInt(userID, 10))
		if err == nil && u.Phone != "" {
			_ = b.DB.UpdateLastLogin(u.ID)
			state.SetState(chatID, state.StateMainMenu)
			b.showMainMenu(ctx, chatID)
			return
		}
		b.handleStart(ctx, chatID, userID)
		return
	}

	st := state.GetCurrentState(chatID)
	if b.tryHandlePhoto(ctx, chatID, upd) {
		return
	}
	if b.tryHandleFile(ctx, chatID, upd) {
		return
	}

	switch st {
	case state.StateWaitPhone:
		b.handlePhoneInput(ctx, chatID, userID, text)
	case state.StateRegName:
		b.handleRegName(ctx, chatID, text)
	case state.StateRegPatronymic:
		b.handleRegPatronymic(ctx, chatID, text)
	case state.StateRegLastName:
		b.handleRegLastName(ctx, chatID, text)
	case state.StateRegWorkArea:
		b.handleRegWorkArea(ctx, chatID, userID, text)
	case state.StateProfileEdit:
		b.handleProfileEdit(ctx, chatID, userID, text)
	case state.StateJ1Date:
		b.handleJ1Date(ctx, chatID, text)
	case state.StateJ1Time:
		b.handleJ1Time(ctx, chatID, text)
	case state.StateJ1HatchDesc:
		b.handleOptionalText(ctx, chatID, text, "hatch_description", state.StateJ1Year, "🔢 Введите год ввода трубопровода в эксплуатацию:")
	case state.StateJ1Year:
		b.handleJ1Year(ctx, chatID, text)
	case state.StateJ2AccessDesc, state.StateJ2GasDesc, state.StateJ2WaterDesc,
		state.StateJ2DrainageDesc, state.StateJ2VentilationDesc, state.StateJ2SiltingDesc,
		state.StateJ3DrainDesc, state.StateJ4EquipDesc:
		b.handleJournalOptionalDesc(ctx, chatID, st, text)
	case state.StateExpertChatMsg:
		b.handleExpertMessage(ctx, chatID, userID, text)
	case state.StateAdminAddExpert:
		b.handleAdminAddExpertPhone(ctx, chatID, text)
	default:
		if isPhotoState(st) {
			b.sendText(ctx, chatID, "🤔 Не похоже на фото. Загрузите изображение")
		}
	}
}

func (b *Bot) handleCallback(ctx context.Context, upd *schemes.MessageCallbackUpdate) {
	cb := upd.Callback
	chatID := int64(0)
	if upd.Message != nil {
		chatID = upd.Message.Recipient.ChatId
	}
	payload := cb.Payload

	_ = b.API.Messages.AnswerOnCallback(ctx, cb.CallbackID, &schemes.CallbackAnswer{
		Notification: "ok",
	})

	if strings.HasPrefix(payload, "admin_approve:") {
		b.handleAdminApprove(ctx, cb.User.UserId, payload)
		return
	}
	if strings.HasPrefix(payload, "admin_reject:") {
		return
	}
	if strings.HasPrefix(payload, "reply:") {
		b.handleExpertReply(ctx, chatID, cb.User.UserId, payload)
		return
	}
	if strings.HasPrefix(payload, "measure_view:") {
		return
	}

	st := state.GetCurrentState(chatID)
	userID := cb.User.UserId

	switch {
	case payload == "action:start":
		b.askPhone(ctx, chatID)
	case payload == "action:exit":
		state.ClearSession(chatID)
		b.sendText(ctx, chatID, "До новых встреч 👋")
	case payload == "action:share_phone":
		state.SetState(chatID, state.StateWaitPhone)
		b.sendText(ctx, chatID, "Введите номер телефона в формате +7XXXXXXXXXX:")
	case payload == "menu:profile":
		b.showProfile(ctx, chatID, userID)
	case payload == "menu:journal":
		b.startJournal(ctx, chatID, userID)
	case payload == "menu:measures":
		b.showMeasures(ctx, chatID, userID)
	case payload == "menu:experts":
		b.showExperts(ctx, chatID)
	case payload == "menu:back":
		state.SetState(chatID, state.StateMainMenu)
		b.showMainMenu(ctx, chatID)
	case payload == "profile:edit":
		state.SetState(chatID, state.StateProfileEdit)
		b.sendText(ctx, chatID, "Введите новый участок работы:")
	case strings.HasPrefix(payload, "position:"):
		b.handleRegPosition(ctx, chatID, strings.TrimPrefix(payload, "position:"))
	case strings.HasPrefix(payload, "experience:"):
		b.handleRegExperience(ctx, chatID, userID, strings.TrimPrefix(payload, "experience:"))
	case strings.HasPrefix(payload, "tk_addr:"):
		b.handleJ1Address(ctx, chatID, strings.TrimPrefix(payload, "tk_addr:"))
	case strings.HasPrefix(payload, "diameter:"):
		b.handleJ1Diameter(ctx, chatID, strings.TrimPrefix(payload, "diameter:"))
	case strings.HasPrefix(payload, "zones:"):
		b.handleJ1Zones(ctx, chatID, strings.TrimPrefix(payload, "zones:"))
	case strings.HasPrefix(payload, "skip:"):
		b.handleSkip(ctx, chatID, st, strings.TrimPrefix(payload, "skip:"))
	case strings.HasPrefix(payload, "access:"):
		b.handleJ2Access(ctx, chatID, strings.TrimPrefix(payload, "access:"))
	case strings.HasPrefix(payload, "gas:"):
		b.handleJ2Gas(ctx, chatID, strings.TrimPrefix(payload, "gas:"))
	case strings.HasPrefix(payload, "water:"):
		b.handleJ2WaterTemp(ctx, chatID, strings.TrimPrefix(payload, "water:"))
	case strings.HasPrefix(payload, "wsource:"):
		b.handleJ2WaterSource(ctx, chatID, strings.TrimPrefix(payload, "wsource:"))
	case strings.HasPrefix(payload, "drainage:"):
		b.handleJ2Drainage(ctx, chatID, strings.TrimPrefix(payload, "drainage:"))
	case strings.HasPrefix(payload, "vent:"):
		b.handleJ2Ventilation(ctx, chatID, strings.TrimPrefix(payload, "vent:"))
	case strings.HasPrefix(payload, "silt:"):
		b.handleJ2Silting(ctx, chatID, strings.TrimPrefix(payload, "silt:"))
	case strings.HasPrefix(payload, "j3_"):
		b.handleJ3Callback(ctx, chatID, payload)
	case strings.HasPrefix(payload, "j4_diam:"):
		b.handleJ4Diameter(ctx, chatID, strings.TrimPrefix(payload, "j4_diam:"))
	case strings.HasPrefix(payload, "j4_insul:"):
		b.handleJ4Insulation(ctx, chatID, strings.TrimPrefix(payload, "j4_insul:"))
	case strings.HasPrefix(payload, "j4_metal_r1:"):
		b.handleJ4Metal(ctx, chatID, "metal_r1", state.StateJ4MetalR1Photo, strings.TrimPrefix(payload, "j4_metal_r1:"))
	case strings.HasPrefix(payload, "j4_metal_r2:"):
		b.handleJ4Metal(ctx, chatID, "metal_r2", state.StateJ4MetalR2Photo, strings.TrimPrefix(payload, "j4_metal_r2:"))
	case strings.HasPrefix(payload, "j4_valve_r1:"):
		b.handleJ4Valve(ctx, chatID, "valve_r1", state.StateJ4ValveR1Photo, strings.TrimPrefix(payload, "j4_valve_r1:"))
	case strings.HasPrefix(payload, "j4_valve_r2:"):
		b.handleJ4Valve(ctx, chatID, "valve_r2", state.StateJ4ValveR2Photo, strings.TrimPrefix(payload, "j4_valve_r2:"))
	case payload == "journal:save":
		b.saveJournal(ctx, chatID, userID)
	case payload == "journal:edit":
		b.showJournalEditMenu(ctx, chatID)
	case payload == "journal:cancel":
		state.SetState(chatID, state.StateMainMenu)
		b.showMainMenu(ctx, chatID)
	case strings.HasPrefix(payload, "journal:section:"):
		b.gotoJournalSection(ctx, chatID, strings.TrimPrefix(payload, "journal:section:"))
	case payload == "measures:save":
		b.saveMeasures(ctx, chatID, userID)
	case payload == "measures:edit":
		b.startJournal(ctx, chatID, userID)
	case payload == "measures:cancel":
		state.SetState(chatID, state.StateMainMenu)
		b.showMainMenu(ctx, chatID)
	case strings.HasPrefix(payload, "expert_chat:"):
		b.startExpertChat(ctx, chatID, strings.TrimPrefix(payload, "expert_chat:"))
	case payload == "experts:list":
		b.showExperts(ctx, chatID)
	case strings.HasPrefix(payload, "admin:"):
		if strings.HasPrefix(payload, "admin:rm_expert:") {
			b.handleAdminRemoveExpert(ctx, chatID, payload)
			return
		}
		if payload == "admin:back" {
			b.showAdminMenu(ctx, chatID)
			return
		}
		b.handleAdminCallback(ctx, chatID, userID, payload)
	}
}

func (b *Bot) sendText(ctx context.Context, chatID int64, text string) {
	msg := maxbot.NewMessage().SetChat(chatID).SetText(text)
	if err := b.API.Messages.Send(ctx, msg); err != nil {
		log.Printf("send error: %v", err)
	}
}

func (b *Bot) sendWithKeyboard(ctx context.Context, chatID int64, text string, build func(*maxbot.Keyboard)) {
	kb := b.API.Messages.NewKeyboardBuilder()
	build(kb)
	msg := maxbot.NewMessage().SetChat(chatID).SetText(text).AddKeyboard(kb)
	if err := b.API.Messages.Send(ctx, msg); err != nil {
		log.Printf("send error: %v", err)
	}
}

func (b *Bot) sendFile(ctx context.Context, chatID int64, path, caption string) error {
	info, err := b.API.Uploads.UploadMediaFromFile(ctx, schemes.FILE, path)
	if err != nil {
		return err
	}
	msg := maxbot.NewMessage().SetChat(chatID).SetText(caption).AddFile(info)
	return b.API.Messages.Send(ctx, msg)
}

func nameRe() *regexp.Regexp {
	return regexp.MustCompile(`^[а-яА-ЯёЁa-zA-Z\s\-]+$`)
}

func phoneRe() *regexp.Regexp {
	return regexp.MustCompile(`^\+7\d{10}$`)
}

func normalizePhone(s string) string {
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, "(", "")
	s = strings.ReplaceAll(s, ")", "")
	if strings.HasPrefix(s, "8") && len(s) == 11 {
		s = "+7" + s[1:]
	}
	if strings.HasPrefix(s, "7") && len(s) == 11 {
		s = "+" + s
	}
	return s
}

func journalFromState(chatID int64) *db.Journal {
	j := &db.Journal{}
	j.InspectionDate = state.GetDataString(chatID, "inspection_date")
	j.InspectionTime = state.GetDataString(chatID, "inspection_time")
	j.TkAddress = state.GetDataString(chatID, "tk_address")
	j.MainDiameter = state.GetDataString(chatID, "main_diameter")
	j.ProtectionZonesViolated = getInt(chatID, "protection_zones_violated")
	j.ProtectionZonesPhoto = state.GetDataString(chatID, "protection_zones_photo")
	j.HatchDescription = state.GetDataString(chatID, "hatch_description")
	j.PipelineYear = getInt(chatID, "pipeline_year")
	j.ServiceLife = getInt(chatID, "service_life")
	j.ServiceGroup = state.GetDataString(chatID, "service_group")
	j.AccessIssue = state.GetDataString(chatID, "access_issue")
	j.AccessPhoto = state.GetDataString(chatID, "access_photo")
	j.AccessDescription = state.GetDataString(chatID, "access_description")
	j.GasHazard = getInt(chatID, "gas_hazard")
	j.GasHazardDesc = state.GetDataString(chatID, "gas_hazard_description")
	j.WaterTemp = state.GetDataString(chatID, "water_temp")
	j.WaterSource = state.GetDataString(chatID, "water_source")
	j.WaterDescription = state.GetDataString(chatID, "water_description")
	j.Drainage = state.GetDataString(chatID, "drainage")
	j.DrainagePhoto = state.GetDataString(chatID, "drainage_photo")
	j.DrainageDesc = state.GetDataString(chatID, "drainage_description")
	j.Ventilation = state.GetDataString(chatID, "ventilation")
	j.VentilationDesc = state.GetDataString(chatID, "ventilation_description")
	j.SiltingLevel = state.GetDataString(chatID, "silting_level")
	j.SiltingDesc = state.GetDataString(chatID, "silting_description")
	j.ConcreteDamaged = getInt(chatID, "concrete_damaged")
	j.ConcreteDamagedPhoto = state.GetDataString(chatID, "concrete_damaged_photo")
	j.ConcreteOk = getInt(chatID, "concrete_ok")
	j.ConcreteOkPhoto = state.GetDataString(chatID, "concrete_ok_photo")
	j.SupportsCorrosion = state.GetDataString(chatID, "supports_corrosion")
	j.SupportsCorrosionPhoto = state.GetDataString(chatID, "supports_corrosion_photo")
	j.SupportsOk = getInt(chatID, "supports_ok")
	j.SupportsOkPhoto = state.GetDataString(chatID, "supports_ok_photo")
	j.PipeR1Corrosion = state.GetDataString(chatID, "pipe_r1_corrosion")
	j.PipeR1CorrosionPhoto = state.GetDataString(chatID, "pipe_r1_corrosion_photo")
	j.PipeR1Ok = getInt(chatID, "pipe_r1_ok")
	j.PipeR1OkPhoto = state.GetDataString(chatID, "pipe_r1_ok_photo")
	j.PipeR2Corrosion = state.GetDataString(chatID, "pipe_r2_corrosion")
	j.PipeR2CorrosionPhoto = state.GetDataString(chatID, "pipe_r2_corrosion_photo")
	j.PipeR2Ok = getInt(chatID, "pipe_r2_ok")
	j.PipeR2OkPhoto = state.GetDataString(chatID, "pipe_r2_ok_photo")
	j.ValveMain = state.GetDataString(chatID, "valve_main")
	j.ValveMainPhoto = state.GetDataString(chatID, "valve_main_photo")
	j.DrainLine = state.GetDataString(chatID, "drain_line")
	j.DrainLinePhoto = state.GetDataString(chatID, "drain_line_photo")
	j.DrainLineDesc = state.GetDataString(chatID, "drain_line_description")
	j.BranchDiameter = state.GetDataString(chatID, "branch_diameter")
	j.InsulationState = state.GetDataString(chatID, "insulation_state")
	j.InsulationPhoto = state.GetDataString(chatID, "insulation_photo")
	j.MetalR1 = state.GetDataString(chatID, "metal_r1")
	j.MetalR1Photo = state.GetDataString(chatID, "metal_r1_photo")
	j.MetalR2 = state.GetDataString(chatID, "metal_r2")
	j.MetalR2Photo = state.GetDataString(chatID, "metal_r2_photo")
	j.ValveR1 = state.GetDataString(chatID, "valve_r1")
	j.ValveR1Photo = state.GetDataString(chatID, "valve_r1_photo")
	j.ValveR2 = state.GetDataString(chatID, "valve_r2")
	j.ValveR2Photo = state.GetDataString(chatID, "valve_r2_photo")
	j.EquipmentDesc = state.GetDataString(chatID, "equipment_description")
	if id := getInt64(chatID, "journal_id"); id > 0 {
		j.ID = id
	}
	return j
}

func getInt(chatID int64, key string) int {
	v := state.GetData(chatID, key)
	if v == nil {
		return 0
	}
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case string:
		i, _ := strconv.Atoi(n)
		return i
	default:
		return 0
	}
}

func getInt64(chatID int64, key string) int64 {
	v := state.GetData(chatID, key)
	if v == nil {
		return 0
	}
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	default:
		return 0
	}
}

func isPhotoState(st string) bool {
	return strings.HasSuffix(st, "_photo")
}

func ensureUploadsDir(dir string) {
	_ = os.MkdirAll(dir, 0755)
}

func gridButtons(kb *maxbot.Keyboard, labels []string, prefix string, perRow int) {
	row := kb.AddRow()
	for i, label := range labels {
		if i > 0 && i%perRow == 0 {
			row = kb.AddRow()
		}
		row.AddCallback(label, schemes.DEFAULT, prefix+label)
	}
}

func formatDate(t time.Time) string {
	return t.Format("02.01.2006")
}

func (b *Bot) getUser(chatID, maxUserID int64) (*db.User, error) {
	return b.DB.GetUserByMaxID(strconv.FormatInt(maxUserID, 10))
}

func (b *Bot) handleOptionalText(ctx context.Context, chatID int64, text, key, nextState, nextPrompt string) {
	state.SetData(chatID, key, text)
	state.SetState(chatID, nextState)
	b.sendText(ctx, chatID, nextPrompt)
}

func skipButton(row *maxbot.KeyboardRow, payload string) {
	row.AddCallback("⏭ Пропустить", schemes.DEFAULT, "skip:"+payload)
}

func photoPromptWithSkip(ctx context.Context, b *Bot, chatID int64, text, skipPayload string) {
	b.sendWithKeyboard(ctx, chatID, text, func(kb *maxbot.Keyboard) {
		skipButton(kb.AddRow(), skipPayload)
	})
}