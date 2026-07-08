package handlers

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"

	maxbot "github.com/max-messenger/max-bot-api-client-go"
	"github.com/max-messenger/max-bot-api-client-go/schemes"

	"kontroler-ts/state"
)

var (
	dateRe = regexp.MustCompile(`^\d{2}\.\d{2}\.\d{4}$`)
	timeRe = regexp.MustCompile(`^\d{2}:\d{2}$`)
)

func (b *Bot) startJournal(ctx context.Context, chatID, userID int64) {
	u, err := b.getUser(chatID, userID)
	if err != nil {
		return
	}
	state.SetData(chatID, "user_id", u.ID)
	state.SetState(chatID, state.StateJ1Date)
	b.sendText(ctx, chatID, "📅 Введите дату начала обхода (дд.мм.гггг):")
}

func (b *Bot) handleJ1Date(ctx context.Context, chatID int64, text string) {
	if !dateRe.MatchString(text) {
		b.sendText(ctx, chatID, "🤔 Не похоже на дату. Попробуйте еще раз")
		return
	}
	state.SetData(chatID, "inspection_date", text)
	state.SetState(chatID, state.StateJ1Time)
	b.sendText(ctx, chatID, "🕐 Введите время начала обхода (чч:мм):")
}

func (b *Bot) handleJ1Time(ctx context.Context, chatID int64, text string) {
	if !timeRe.MatchString(text) {
		b.sendText(ctx, chatID, "🤔 Не похоже на время. Попробуйте еще раз")
		return
	}
	state.SetData(chatID, "inspection_time", text)
	state.SetState(chatID, state.StateJ1Address)
	addrs, _ := b.DB.GetTKAddresses()
	b.sendWithKeyboard(ctx, chatID, "📍 Укажите полицейский адрес обхода и номер ТК:", func(kb *maxbot.Keyboard) {
		for _, a := range addrs {
			kb.AddRow().AddCallback(a.Address, schemes.DEFAULT, "tk_addr:"+a.Address)
		}
	})
}

func (b *Bot) handleJ1Address(ctx context.Context, chatID int64, addr string) {
	state.SetData(chatID, "tk_address", addr)
	state.SetState(chatID, state.StateJ1Diameter)
	diams := []string{"50", "70", "80", "100", "125", "150", "200", "300", "500", "600", "700", "800", "1000", "1200", "1400"}
	b.sendWithKeyboard(ctx, chatID, "📏 Укажите диаметр (мм) основной магистрали:", func(kb *maxbot.Keyboard) {
		gridButtons(kb, diams, "diameter:", 4)
	})
}

func (b *Bot) handleJ1Diameter(ctx context.Context, chatID int64, d string) {
	state.SetData(chatID, "main_diameter", d)
	state.SetState(chatID, state.StateJ1Zones)
	b.sendWithKeyboard(ctx, chatID, "🚧 Охранные зоны нарушены?", func(kb *maxbot.Keyboard) {
		kb.AddRow().
			AddCallback("😔 Да", schemes.DEFAULT, "zones:yes").
			AddCallback("😊 Нет", schemes.DEFAULT, "zones:no")
	})
}

func (b *Bot) handleJ1Zones(ctx context.Context, chatID int64, val string) {
	if val == "yes" {
		state.SetData(chatID, "protection_zones_violated", 1)
	} else {
		state.SetData(chatID, "protection_zones_violated", 0)
	}
	state.SetState(chatID, state.StateJ1ZonesPhoto)
	photoPromptWithSkip(ctx, b, chatID, "📷 Добавьте фото для подтверждения (jpg/jpeg/png):", "zones_photo")
}

func (b *Bot) handleJ1Year(ctx context.Context, chatID int64, text string) {
	year, err := strconv.Atoi(text)
	if err != nil || len(text) != 4 || year < 1900 || year > time.Now().Year() {
		b.sendText(ctx, chatID, "🤔 Не похоже на год. Попробуйте еще раз")
		return
	}
	state.SetData(chatID, "pipeline_year", year)

	insDate := state.GetDataString(chatID, "inspection_date")
	insYear := time.Now().Year()
	if parts := strings.Split(insDate, "."); len(parts) == 3 {
		if y, e := strconv.Atoi(parts[2]); e == nil {
			insYear = y
		}
	}
	life := insYear - year
	state.SetData(chatID, "service_life", life)

	var groupMsg string
	switch {
	case life <= 5:
		groupMsg = "I группа по сроку службы «Шнурок» (низкий риск повреждений) 😊"
		state.SetData(chatID, "service_group", "I")
	case life <= 15:
		groupMsg = "II группа «Черпак» (средний риск повреждений) 😊"
		state.SetData(chatID, "service_group", "II")
	default:
		groupMsg = "III группа «Король» (высокий риск повреждений) 😊"
		state.SetData(chatID, "service_group", "III")
	}
	b.sendText(ctx, chatID, groupMsg)

	go func() {
		time.Sleep(5 * time.Second)
		b.startSection2(context.Background(), chatID)
	}()
}

func (b *Bot) startSection2(ctx context.Context, chatID int64) {
	state.SetState(chatID, state.StateJ2Access)
	b.sendWithKeyboard(ctx, chatID, "🚫 Укажите отсутствие доступа:", func(kb *maxbot.Keyboard) {
		kb.AddRow().AddCallback("😊 Не обнаружено", schemes.DEFAULT, "access:none")
		kb.AddRow().AddCallback("💧 Затоплена", schemes.DEFAULT, "access:flooded")
		kb.AddRow().AddCallback("♨️ Запарена", schemes.DEFAULT, "access:steamed")
		kb.AddRow().AddCallback("🟤 Заиливание", schemes.DEFAULT, "access:silted")
	})
}

func (b *Bot) handleJ2Access(ctx context.Context, chatID int64, val string) {
	state.SetData(chatID, "access_issue", val)
	state.SetData(chatID, "j2_branch", val)
	state.SetState(chatID, state.StateJ2AccessPhoto)
	photoPromptWithSkip(ctx, b, chatID, "📷 Добавьте фото (необязательно):", "access_photo")
}

func (b *Bot) afterJ2AccessPhoto(ctx context.Context, chatID int64) {
	state.SetState(chatID, state.StateJ2AccessDesc)
	b.sendWithKeyboard(ctx, chatID, "📝 Описание (необязательно):", func(kb *maxbot.Keyboard) {
		skipButton(kb.AddRow(), "access_desc")
	})
}

func (b *Bot) afterJ2AccessDesc(ctx context.Context, chatID int64) {
	branch := state.GetDataString(chatID, "j2_branch")
	switch branch {
	case "none":
		b.askJ2Gas(ctx, chatID)
	case "flooded":
		state.SetState(chatID, state.StateJ2WaterTemp)
		b.sendWithKeyboard(ctx, chatID, "💧 Вода в ТК?", func(kb *maxbot.Keyboard) {
			kb.AddRow().
				AddCallback("🤔 Холодная", schemes.DEFAULT, "water:cold").
				AddCallback("😔 Горячая", schemes.DEFAULT, "water:hot")
		})
	case "steamed":
		state.SetState(chatID, state.StateJ2Ventilation)
		b.sendWithKeyboard(ctx, chatID, "🌬 Состояние вентиляции/дефлекторов:", func(kb *maxbot.Keyboard) {
			kb.AddRow().AddCallback("Работает", schemes.DEFAULT, "vent:works")
			kb.AddRow().AddCallback("Отсутствует по проекту", schemes.DEFAULT, "vent:missing")
			kb.AddRow().AddCallback("Не обнаружен", schemes.DEFAULT, "vent:notfound")
			kb.AddRow().AddCallback("Не эффективно", schemes.DEFAULT, "vent:ineffective")
			kb.AddRow().AddCallback("Требуется чистка", schemes.DEFAULT, "vent:clean")
		})
	case "silted":
		state.SetState(chatID, state.StateJ2Silting)
		b.sendWithKeyboard(ctx, chatID, "🟤 Заиливание:", func(kb *maxbot.Keyboard) {
			kb.AddRow().AddCallback("Ниже изоляции", schemes.DEFAULT, "silt:below")
			kb.AddRow().AddCallback("Касание изоляции", schemes.DEFAULT, "silt:touch")
			kb.AddRow().AddCallback("Полностью по горловину", schemes.DEFAULT, "silt:full")
		})
	}
}

func (b *Bot) askJ2Gas(ctx context.Context, chatID int64) {
	state.SetState(chatID, state.StateJ2Gas)
	b.sendWithKeyboard(ctx, chatID, "⚠️ Газоопасная ТК?", func(kb *maxbot.Keyboard) {
		kb.AddRow().
			AddCallback("🤔 Да", schemes.DEFAULT, "gas:yes").
			AddCallback("😊 Нет", schemes.DEFAULT, "gas:no")
	})
}

func (b *Bot) handleJ2Gas(ctx context.Context, chatID int64, val string) {
	if val == "yes" {
		state.SetData(chatID, "gas_hazard", 1)
	} else {
		state.SetData(chatID, "gas_hazard", 0)
	}
	state.SetState(chatID, state.StateJ2GasDesc)
	b.sendWithKeyboard(ctx, chatID, "📝 Описание (необязательно):", func(kb *maxbot.Keyboard) {
		skipButton(kb.AddRow(), "gas_desc")
	})
}

func (b *Bot) afterJ2GasDesc(ctx context.Context, chatID int64) {
	b.askJ2Drainage(ctx, chatID)
}

func (b *Bot) askJ2Drainage(ctx context.Context, chatID int64) {
	state.SetState(chatID, state.StateJ2Drainage)
	b.sendWithKeyboard(ctx, chatID, "🔧 Сопутствующий дренаж:", func(kb *maxbot.Keyboard) {
		kb.AddRow().AddCallback("Работает", schemes.DEFAULT, "drainage:works")
		kb.AddRow().AddCallback("Отсутствует по проекту", schemes.DEFAULT, "drainage:missing")
		kb.AddRow().AddCallback("Не обнаружен", schemes.DEFAULT, "drainage:notfound")
		kb.AddRow().AddCallback("Требуется чистка", schemes.DEFAULT, "drainage:clean")
		kb.AddRow().AddCallback("Требуется ремонт", schemes.DEFAULT, "drainage:repair")
	})
}

func (b *Bot) handleJ2Drainage(ctx context.Context, chatID int64, val string) {
	state.SetData(chatID, "drainage", val)
	state.SetState(chatID, state.StateJ2DrainagePhoto)
	photoPromptWithSkip(ctx, b, chatID, "📷 Фото дренажа (необязательно):", "drainage_photo")
}

func (b *Bot) afterJ2DrainagePhoto(ctx context.Context, chatID int64) {
	state.SetState(chatID, state.StateJ2DrainageDesc)
	b.sendWithKeyboard(ctx, chatID, "📝 Описание (необязательно):", func(kb *maxbot.Keyboard) {
		skipButton(kb.AddRow(), "drainage_desc")
	})
}

func (b *Bot) afterJ2DrainageDesc(ctx context.Context, chatID int64) {
	b.startSection3(ctx, chatID)
}

func (b *Bot) handleJ2WaterTemp(ctx context.Context, chatID int64, val string) {
	state.SetData(chatID, "water_temp", val)
	state.SetState(chatID, state.StateJ2WaterSource)
	if val == "cold" {
		b.sendWithKeyboard(ctx, chatID, "Источник воды:", func(kb *maxbot.Keyboard) {
			kb.AddRow().AddCallback("Грунтовая вода", schemes.DEFAULT, "wsource:ground")
			kb.AddRow().AddCallback("Поверхностная вода", schemes.DEFAULT, "wsource:surface")
			kb.AddRow().AddCallback("Не определено", schemes.DEFAULT, "wsource:unknown")
		})
	} else {
		b.sendWithKeyboard(ctx, chatID, "Источник воды:", func(kb *maxbot.Keyboard) {
			kb.AddRow().AddCallback("Из дренажа", schemes.DEFAULT, "wsource:drain")
			kb.AddRow().AddCallback("Из простенка/гильзы", schemes.DEFAULT, "wsource:wall")
			kb.AddRow().AddCallback("Свищ в камере", schemes.DEFAULT, "wsource:fistula")
			kb.AddRow().AddCallback("Течёт сальник", schemes.DEFAULT, "wsource:gland")
			kb.AddRow().AddCallback("Течёт компенсатор", schemes.DEFAULT, "wsource:comp")
			kb.AddRow().AddCallback("Не определено", schemes.DEFAULT, "wsource:unknown")
		})
	}
}

func (b *Bot) handleJ2WaterSource(ctx context.Context, chatID int64, val string) {
	state.SetData(chatID, "water_source", val)
	state.SetState(chatID, state.StateJ2WaterDesc)
	b.sendWithKeyboard(ctx, chatID, "📝 Описание (необязательно):", func(kb *maxbot.Keyboard) {
		skipButton(kb.AddRow(), "water_desc")
	})
}

func (b *Bot) afterJ2WaterDesc(ctx context.Context, chatID int64) {
	b.askJ2Drainage(ctx, chatID)
}

func (b *Bot) handleJ2Ventilation(ctx context.Context, chatID int64, val string) {
	state.SetData(chatID, "ventilation", val)
	state.SetState(chatID, state.StateJ2VentilationDesc)
	b.sendWithKeyboard(ctx, chatID, "📝 Описание (необязательно):", func(kb *maxbot.Keyboard) {
		skipButton(kb.AddRow(), "ventilation_desc")
	})
}

func (b *Bot) afterJ2VentilationDesc(ctx context.Context, chatID int64) {
	b.startSection3(ctx, chatID)
}

func (b *Bot) handleJ2Silting(ctx context.Context, chatID int64, val string) {
	state.SetData(chatID, "silting_level", val)
	state.SetState(chatID, state.StateJ2SiltingDesc)
	b.sendWithKeyboard(ctx, chatID, "📝 Описание (необязательно):", func(kb *maxbot.Keyboard) {
		skipButton(kb.AddRow(), "silting_desc")
	})
}

func (b *Bot) afterJ2SiltingDesc(ctx context.Context, chatID int64) {
	b.startSection3(ctx, chatID)
}

func (b *Bot) startSection3(ctx context.Context, chatID int64) {
	state.SetState(chatID, state.StateJ3ConcreteCorr)
	b.sendWithKeyboard(ctx, chatID, "🏗 Бетонные конструкции разрушены (видна арматура)?", func(kb *maxbot.Keyboard) {
		kb.AddRow().
			AddCallback("😔 Да", schemes.DEFAULT, "j3_concrete_dmg:yes").
			AddCallback("😊 Нет", schemes.DEFAULT, "j3_concrete_dmg:no")
	})
}

func (b *Bot) handleJ3Callback(ctx context.Context, chatID int64, payload string) {
	parts := strings.SplitN(payload, ":", 2)
	if len(parts) != 2 {
		return
	}
	key, val := parts[0], parts[1]

	type step struct {
		dataKey   string
		intVal    bool
		yesInt    int
		photoState string
		photoKey   string
		nextAsk    func(context.Context, int64)
	}

	steps := map[string]step{
		"j3_concrete_dmg": {dataKey: "concrete_damaged", intVal: true, yesInt: 1, photoState: state.StateJ3ConcreteCorrPhoto, photoKey: "concrete_damaged_photo", nextAsk: func(c context.Context, id int64) {
			state.SetState(id, state.StateJ3ConcreteOk)
			b.sendWithKeyboard(c, id, "✅ Конструкции в хорошем состоянии?", func(kb *maxbot.Keyboard) {
				kb.AddRow().AddCallback("😊 Да", schemes.DEFAULT, "j3_concrete_ok:yes").AddCallback("😔 Нет", schemes.DEFAULT, "j3_concrete_ok:no")
			})
		}},
		"j3_concrete_ok": {dataKey: "concrete_ok", intVal: true, yesInt: 1, photoState: state.StateJ3ConcreteOkPhoto, photoKey: "concrete_ok_photo", nextAsk: func(c context.Context, id int64) {
			state.SetState(id, state.StateJ3SupportsCorr)
			b.sendWithKeyboard(c, id, "🔩 Равномерная коррозия опор?", func(kb *maxbot.Keyboard) {
				kb.AddRow().AddCallback("😔 Да", schemes.DEFAULT, "j3_supports_corr:yes").AddCallback("😊 Нет, коррозии", schemes.DEFAULT, "j3_supports_corr:no")
			})
		}},
		"j3_supports_corr": {dataKey: "supports_corrosion", photoState: state.StateJ3SupportsCorrPhoto, photoKey: "supports_corrosion_photo", nextAsk: func(c context.Context, id int64) {
			state.SetState(id, state.StateJ3SupportsOk)
			b.sendWithKeyboard(c, id, "✅ Опоры в хорошем состоянии?", func(kb *maxbot.Keyboard) {
				kb.AddRow().AddCallback("😊 Да", schemes.DEFAULT, "j3_supports_ok:yes").AddCallback("😔 Нет", schemes.DEFAULT, "j3_supports_ok:no")
			})
		}},
		"j3_supports_ok": {dataKey: "supports_ok", intVal: true, yesInt: 1, photoState: state.StateJ3SupportsOkPhoto, photoKey: "supports_ok_photo", nextAsk: func(c context.Context, id int64) {
			state.SetState(id, state.StateJ3PipeR1Corr)
			b.sendWithKeyboard(c, id, "🔩 Коррозия металла подающего трубопровода (Р1)?", func(kb *maxbot.Keyboard) {
				kb.AddRow().AddCallback("😔 Да", schemes.DEFAULT, "j3_pipe_r1_corr:yes")
				kb.AddRow().AddCallback("😊 Нет, коррозии", schemes.DEFAULT, "j3_pipe_r1_corr:no")
				kb.AddRow().AddCallback("🤔 Изоляция (нет доступа)", schemes.DEFAULT, "j3_pipe_r1_corr:insulation")
			})
		}},
		"j3_pipe_r1_corr": {dataKey: "pipe_r1_corrosion", photoState: state.StateJ3PipeR1CorrPhoto, photoKey: "pipe_r1_corrosion_photo", nextAsk: func(c context.Context, id int64) {
			state.SetState(id, state.StateJ3PipeR1Ok)
			b.sendWithKeyboard(c, id, "✅ Металл (Р1) в хорошем состоянии?", func(kb *maxbot.Keyboard) {
				kb.AddRow().AddCallback("😊 Да", schemes.DEFAULT, "j3_pipe_r1_ok:yes").AddCallback("😔 Нет", schemes.DEFAULT, "j3_pipe_r1_ok:no")
			})
		}},
		"j3_pipe_r1_ok": {dataKey: "pipe_r1_ok", intVal: true, yesInt: 1, photoState: state.StateJ3PipeR1OkPhoto, photoKey: "pipe_r1_ok_photo", nextAsk: func(c context.Context, id int64) {
			state.SetState(id, state.StateJ3PipeR2Corr)
			b.sendWithKeyboard(c, id, "🔩 Коррозия металла обратного трубопровода (Р2)?", func(kb *maxbot.Keyboard) {
				kb.AddRow().AddCallback("😔 Да", schemes.DEFAULT, "j3_pipe_r2_corr:yes")
				kb.AddRow().AddCallback("😊 Нет, коррозии", schemes.DEFAULT, "j3_pipe_r2_corr:no")
				kb.AddRow().AddCallback("🤔 Изоляция (нет доступа)", schemes.DEFAULT, "j3_pipe_r2_corr:insulation")
			})
		}},
		"j3_pipe_r2_corr": {dataKey: "pipe_r2_corrosion", photoState: state.StateJ3PipeR2CorrPhoto, photoKey: "pipe_r2_corrosion_photo", nextAsk: func(c context.Context, id int64) {
			state.SetState(id, state.StateJ3PipeR2Ok)
			b.sendWithKeyboard(c, id, "✅ Металл (Р2) в хорошем состоянии?", func(kb *maxbot.Keyboard) {
				kb.AddRow().AddCallback("😊 Да", schemes.DEFAULT, "j3_pipe_r2_ok:yes").AddCallback("😔 Нет", schemes.DEFAULT, "j3_pipe_r2_ok:no")
			})
		}},
		"j3_pipe_r2_ok": {dataKey: "pipe_r2_ok", intVal: true, yesInt: 1, photoState: state.StateJ3PipeR2OkPhoto, photoKey: "pipe_r2_ok_photo", nextAsk: func(c context.Context, id int64) {
			state.SetState(id, state.StateJ3Valve)
			b.sendWithKeyboard(c, id, "🔧 Запорная арматура (магистральные трубопроводы):", func(kb *maxbot.Keyboard) {
				kb.AddRow().AddCallback("🤔 Требуется замена сальника", schemes.DEFAULT, "j3_valve:gland")
				kb.AddRow().AddCallback("😔 Требуется замена арматуры", schemes.DEFAULT, "j3_valve:replace")
				kb.AddRow().AddCallback("😊 Хорошее состояние", schemes.DEFAULT, "j3_valve:ok")
			})
		}},
		"j3_valve": {dataKey: "valve_main", photoState: state.StateJ3ValvePhoto, photoKey: "valve_main_photo", nextAsk: func(c context.Context, id int64) {
			state.SetState(id, state.StateJ3Drain)
			b.sendWithKeyboard(c, id, "📉 Состояние спускной линии:", func(kb *maxbot.Keyboard) {
				kb.AddRow().AddCallback("😔 Коррозия (грунт/вода касается металла)", schemes.DEFAULT, "j3_drain:corrosion")
				kb.AddRow().AddCallback("😊 Нет коррозии", schemes.DEFAULT, "j3_drain:ok")
				kb.AddRow().AddCallback("🤔 Подтоплена/заилена (нет доступа)", schemes.DEFAULT, "j3_drain:flooded")
			})
		}},
		"j3_drain": {dataKey: "drain_line", photoState: state.StateJ3DrainPhoto, photoKey: "drain_line_photo", nextAsk: func(c context.Context, id int64) {
			state.SetState(id, state.StateJ3DrainDesc)
			b.sendWithKeyboard(c, id, "📝 Описание (необязательно):", func(kb *maxbot.Keyboard) {
				skipButton(kb.AddRow(), "drain_desc")
			})
		}},
	}

	s, ok := steps[key]
	if !ok {
		return
	}
	if s.intVal {
		if val == "yes" {
			state.SetData(chatID, s.dataKey, s.yesInt)
		} else {
			state.SetData(chatID, s.dataKey, 0)
		}
	} else {
		state.SetData(chatID, s.dataKey, val)
	}
	state.SetState(chatID, s.photoState)
	state.SetData(chatID, "j3_photo_key", s.photoKey)
	state.SetData(chatID, "j3_next_key", key)
	photoPromptWithSkip(ctx, b, chatID, "📷 Добавьте фото:", s.photoKey)
}

func (b *Bot) afterJ3Photo(ctx context.Context, chatID int64) {
	nextKey := state.GetDataString(chatID, "j3_next_key")
	steps := j3NextSteps(b)
	if fn, ok := steps[nextKey]; ok {
		fn(ctx, chatID)
	}
}

func j3NextSteps(b *Bot) map[string]func(context.Context, int64) {
	return map[string]func(context.Context, int64){
		"j3_concrete_dmg": func(c context.Context, id int64) {
			state.SetState(id, state.StateJ3ConcreteOk)
			b.sendWithKeyboard(c, id, "✅ Конструкции в хорошем состоянии?", func(kb *maxbot.Keyboard) {
				kb.AddRow().AddCallback("😊 Да", schemes.DEFAULT, "j3_concrete_ok:yes").AddCallback("😔 Нет", schemes.DEFAULT, "j3_concrete_ok:no")
			})
		},
		"j3_concrete_ok": func(c context.Context, id int64) {
			state.SetState(id, state.StateJ3SupportsCorr)
			b.sendWithKeyboard(c, id, "🔩 Равномерная коррозия опор?", func(kb *maxbot.Keyboard) {
				kb.AddRow().AddCallback("😔 Да", schemes.DEFAULT, "j3_supports_corr:yes").AddCallback("😊 Нет, коррозии", schemes.DEFAULT, "j3_supports_corr:no")
			})
		},
		"j3_supports_corr": func(c context.Context, id int64) {
			state.SetState(id, state.StateJ3SupportsOk)
			b.sendWithKeyboard(c, id, "✅ Опоры в хорошем состоянии?", func(kb *maxbot.Keyboard) {
				kb.AddRow().AddCallback("😊 Да", schemes.DEFAULT, "j3_supports_ok:yes").AddCallback("😔 Нет", schemes.DEFAULT, "j3_supports_ok:no")
			})
		},
		"j3_supports_ok": func(c context.Context, id int64) {
			state.SetState(id, state.StateJ3PipeR1Corr)
			b.sendWithKeyboard(c, id, "🔩 Коррозия металла подающего трубопровода (Р1)?", func(kb *maxbot.Keyboard) {
				kb.AddRow().AddCallback("😔 Да", schemes.DEFAULT, "j3_pipe_r1_corr:yes")
				kb.AddRow().AddCallback("😊 Нет, коррозии", schemes.DEFAULT, "j3_pipe_r1_corr:no")
				kb.AddRow().AddCallback("🤔 Изоляция (нет доступа)", schemes.DEFAULT, "j3_pipe_r1_corr:insulation")
			})
		},
		"j3_pipe_r1_corr": func(c context.Context, id int64) {
			state.SetState(id, state.StateJ3PipeR1Ok)
			b.sendWithKeyboard(c, id, "✅ Металл (Р1) в хорошем состоянии?", func(kb *maxbot.Keyboard) {
				kb.AddRow().AddCallback("😊 Да", schemes.DEFAULT, "j3_pipe_r1_ok:yes").AddCallback("😔 Нет", schemes.DEFAULT, "j3_pipe_r1_ok:no")
			})
		},
		"j3_pipe_r1_ok": func(c context.Context, id int64) {
			state.SetState(id, state.StateJ3PipeR2Corr)
			b.sendWithKeyboard(c, id, "🔩 Коррозия металла обратного трубопровода (Р2)?", func(kb *maxbot.Keyboard) {
				kb.AddRow().AddCallback("😔 Да", schemes.DEFAULT, "j3_pipe_r2_corr:yes")
				kb.AddRow().AddCallback("😊 Нет, коррозии", schemes.DEFAULT, "j3_pipe_r2_corr:no")
				kb.AddRow().AddCallback("🤔 Изоляция (нет доступа)", schemes.DEFAULT, "j3_pipe_r2_corr:insulation")
			})
		},
		"j3_pipe_r2_corr": func(c context.Context, id int64) {
			state.SetState(id, state.StateJ3PipeR2Ok)
			b.sendWithKeyboard(c, id, "✅ Металл (Р2) в хорошем состоянии?", func(kb *maxbot.Keyboard) {
				kb.AddRow().AddCallback("😊 Да", schemes.DEFAULT, "j3_pipe_r2_ok:yes").AddCallback("😔 Нет", schemes.DEFAULT, "j3_pipe_r2_ok:no")
			})
		},
		"j3_pipe_r2_ok": func(c context.Context, id int64) {
			state.SetState(id, state.StateJ3Valve)
			b.sendWithKeyboard(c, id, "🔧 Запорная арматура (магистральные трубопроводы):", func(kb *maxbot.Keyboard) {
				kb.AddRow().AddCallback("🤔 Требуется замена сальника", schemes.DEFAULT, "j3_valve:gland")
				kb.AddRow().AddCallback("😔 Требуется замена арматуры", schemes.DEFAULT, "j3_valve:replace")
				kb.AddRow().AddCallback("😊 Хорошее состояние", schemes.DEFAULT, "j3_valve:ok")
			})
		},
		"j3_valve": func(c context.Context, id int64) {
			state.SetState(id, state.StateJ3Drain)
			b.sendWithKeyboard(c, id, "📉 Состояние спускной линии:", func(kb *maxbot.Keyboard) {
				kb.AddRow().AddCallback("😔 Коррозия (грунт/вода касается металла)", schemes.DEFAULT, "j3_drain:corrosion")
				kb.AddRow().AddCallback("😊 Нет коррозии", schemes.DEFAULT, "j3_drain:ok")
				kb.AddRow().AddCallback("🤔 Подтоплена/заилена (нет доступа)", schemes.DEFAULT, "j3_drain:flooded")
			})
		},
		"j3_drain": func(c context.Context, id int64) {
			state.SetState(id, state.StateJ3DrainDesc)
			b.sendWithKeyboard(c, id, "📝 Описание (необязательно):", func(kb *maxbot.Keyboard) {
				skipButton(kb.AddRow(), "drain_desc")
			})
		},
	}
}

func (b *Bot) startSection4(ctx context.Context, chatID int64) {
	state.SetState(chatID, state.StateJ4Diameter)
	diams := []string{"50", "70", "80", "100", "125", "150", "200", "250"}
	b.sendWithKeyboard(ctx, chatID, "📏 Диаметр подающего и обратного ввода (мм):", func(kb *maxbot.Keyboard) {
		gridButtons(kb, diams, "j4_diam:", 4)
	})
}

func (b *Bot) handleJ4Diameter(ctx context.Context, chatID int64, d string) {
	state.SetData(chatID, "branch_diameter", d)
	state.SetState(chatID, state.StateJ4Insulation)
	b.sendWithKeyboard(ctx, chatID, "🌡 Состояние тепловой изоляции:", func(kb *maxbot.Keyboard) {
		kb.AddRow().AddCallback("😊 Хорошее", schemes.DEFAULT, "j4_insul:good")
		kb.AddRow().AddCallback("😔 Частично разрушена", schemes.DEFAULT, "j4_insul:partial")
		kb.AddRow().AddCallback("🤔 Отсутствует", schemes.DEFAULT, "j4_insul:missing")
	})
}

func (b *Bot) handleJ4Insulation(ctx context.Context, chatID int64, val string) {
	state.SetData(chatID, "insulation_state", val)
	state.SetState(chatID, state.StateJ4InsulationPhoto)
	photoPromptWithSkip(ctx, b, chatID, "📷 Фото изоляции:", "insulation_photo")
}

func (b *Bot) afterJ4InsulationPhoto(ctx context.Context, chatID int64) {
	state.SetState(chatID, state.StateJ4MetalR1)
	b.sendWithKeyboard(ctx, chatID, "🔍 Состояние металла на вводах (Р1):", func(kb *maxbot.Keyboard) {
		kb.AddRow().AddCallback("🤔 менее 20%", schemes.DEFAULT, "j4_metal_r1:lt20")
		kb.AddRow().AddCallback("😔 более 20%", schemes.DEFAULT, "j4_metal_r1:gt20")
		kb.AddRow().AddCallback("😊 коррозия отсутствует", schemes.DEFAULT, "j4_metal_r1:none")
	})
}

func (b *Bot) handleJ4Metal(ctx context.Context, chatID int64, key, photoState, val string) {
	state.SetData(chatID, key, val)
	state.SetState(chatID, photoState)
	state.SetData(chatID, "j4_photo_key", key+"_photo")
	photoPromptWithSkip(ctx, b, chatID, "📷 Добавьте фото:", key+"_photo")
}

func (b *Bot) afterJ4MetalR1Photo(ctx context.Context, chatID int64) {
	state.SetState(chatID, state.StateJ4MetalR2)
	b.sendWithKeyboard(ctx, chatID, "🔍 Состояние металла на вводах (Р2):", func(kb *maxbot.Keyboard) {
		kb.AddRow().AddCallback("🤔 менее 20%", schemes.DEFAULT, "j4_metal_r2:lt20")
		kb.AddRow().AddCallback("😔 более 20%", schemes.DEFAULT, "j4_metal_r2:gt20")
		kb.AddRow().AddCallback("😊 коррозия отсутствует", schemes.DEFAULT, "j4_metal_r2:none")
	})
}

func (b *Bot) afterJ4MetalR2Photo(ctx context.Context, chatID int64) {
	state.SetState(chatID, state.StateJ4ValveR1)
	b.sendWithKeyboard(ctx, chatID, "🔧 Запорная арматура на вводах (Р1):", func(kb *maxbot.Keyboard) {
		kb.AddRow().AddCallback("🤔 Замена сальника", schemes.DEFAULT, "j4_valve_r1:gland")
		kb.AddRow().AddCallback("😔 Замена арматуры", schemes.DEFAULT, "j4_valve_r1:replace")
		kb.AddRow().AddCallback("😊 Хорошее состояние", schemes.DEFAULT, "j4_valve_r1:ok")
	})
}

func (b *Bot) handleJ4Valve(ctx context.Context, chatID int64, key, photoState, val string) {
	state.SetData(chatID, key, val)
	state.SetState(chatID, photoState)
	state.SetData(chatID, "j4_valve_key", key)
	photoPromptWithSkip(ctx, b, chatID, "📷 Добавьте фото:", key+"_photo")
}

func (b *Bot) afterJ4ValveR1Photo(ctx context.Context, chatID int64) {
	state.SetState(chatID, state.StateJ4ValveR2)
	b.sendWithKeyboard(ctx, chatID, "🔧 Запорная арматура на вводах (Р2):", func(kb *maxbot.Keyboard) {
		kb.AddRow().AddCallback("🤔 Замена сальника", schemes.DEFAULT, "j4_valve_r2:gland")
		kb.AddRow().AddCallback("😔 Замена арматуры", schemes.DEFAULT, "j4_valve_r2:replace")
		kb.AddRow().AddCallback("😊 Хорошее состояние", schemes.DEFAULT, "j4_valve_r2:ok")
	})
}

func (b *Bot) afterJ4ValveR2Photo(ctx context.Context, chatID int64) {
	state.SetState(chatID, state.StateJ4EquipDesc)
	b.sendWithKeyboard(ctx, chatID, "📝 Описание состояния оборудования (необязательно):", func(kb *maxbot.Keyboard) {
		skipButton(kb.AddRow(), "equip_desc")
	})
}

func (b *Bot) showSaveConfirm(ctx context.Context, chatID int64) {
	state.SetState(chatID, state.StateJSaveConfirm)
	b.sendWithKeyboard(ctx, chatID, "💾 Сохранить журнал обходов?", func(kb *maxbot.Keyboard) {
		kb.AddRow().AddCallback("✅ Сохранить", schemes.DEFAULT, "journal:save")
		kb.AddRow().AddCallback("✏️ Редактировать", schemes.DEFAULT, "journal:edit")
		kb.AddRow().AddCallback("❌ Отмена", schemes.DEFAULT, "journal:cancel")
	})
}

func (b *Bot) showJournalEditMenu(ctx context.Context, chatID int64) {
	b.sendWithKeyboard(ctx, chatID, "Выберите раздел для редактирования:", func(kb *maxbot.Keyboard) {
		kb.AddRow().AddCallback("Раздел 1", schemes.DEFAULT, "journal:section:1")
		kb.AddRow().AddCallback("Раздел 2", schemes.DEFAULT, "journal:section:2")
		kb.AddRow().AddCallback("Раздел 3", schemes.DEFAULT, "journal:section:3")
		kb.AddRow().AddCallback("Раздел 4", schemes.DEFAULT, "journal:section:4")
	})
}

func (b *Bot) gotoJournalSection(ctx context.Context, chatID int64, section string) {
	switch section {
	case "1":
		state.SetState(chatID, state.StateJ1Date)
		b.sendText(ctx, chatID, "📅 Введите дату начала обхода (дд.мм.гггг):")
	case "2":
		b.startSection2(ctx, chatID)
	case "3":
		b.startSection3(ctx, chatID)
	case "4":
		b.startSection4(ctx, chatID)
	}
}

func (b *Bot) saveJournal(ctx context.Context, chatID, userID int64) {
	u, err := b.getUser(chatID, userID)
	if err != nil {
		return
	}
	j := journalFromState(chatID)
	j.UserID = u.ID
	j.Status = "saved"
	id, err := b.DB.SaveJournal(j)
	if err != nil {
		b.sendText(ctx, chatID, "Ошибка сохранения: "+err.Error())
		return
	}
	state.SetData(chatID, "journal_id", id)
	state.SetState(chatID, state.StateMeasures)
	b.showMeasures(ctx, chatID, userID)
}

func (b *Bot) handleJournalOptionalDesc(ctx context.Context, chatID int64, st, text string) {
	key := descKeyForState(st)
	state.SetData(chatID, key, text)
	b.advanceAfterDesc(ctx, chatID, st)
}

func (b *Bot) handleSkip(ctx context.Context, chatID int64, st, skipKey string) {
	b.advanceAfterPhotoSkip(ctx, chatID, st, skipKey)
}

func descKeyForState(st string) string {
	m := map[string]string{
		state.StateJ2AccessDesc:     "access_description",
		state.StateJ2GasDesc:        "gas_hazard_description",
		state.StateJ2WaterDesc:      "water_description",
		state.StateJ2DrainageDesc:   "drainage_description",
		state.StateJ2VentilationDesc: "ventilation_description",
		state.StateJ2SiltingDesc:    "silting_description",
		state.StateJ3DrainDesc:      "drain_line_description",
		state.StateJ4EquipDesc:      "equipment_description",
	}
	return m[st]
}

func (b *Bot) advanceAfterDesc(ctx context.Context, chatID int64, st string) {
	switch st {
	case state.StateJ2AccessDesc:
		b.afterJ2AccessDesc(ctx, chatID)
	case state.StateJ2GasDesc:
		b.afterJ2GasDesc(ctx, chatID)
	case state.StateJ2WaterDesc:
		b.afterJ2WaterDesc(ctx, chatID)
	case state.StateJ2DrainageDesc:
		b.afterJ2DrainageDesc(ctx, chatID)
	case state.StateJ2VentilationDesc:
		b.afterJ2VentilationDesc(ctx, chatID)
	case state.StateJ2SiltingDesc:
		b.afterJ2SiltingDesc(ctx, chatID)
	case state.StateJ3DrainDesc:
		b.startSection4(ctx, chatID)
	case state.StateJ4EquipDesc:
		b.showSaveConfirm(ctx, chatID)
	}
}

func (b *Bot) advanceAfterPhotoSkip(ctx context.Context, chatID int64, st, skipKey string) {
	switch skipKey {
	case "zones_photo":
		state.SetState(chatID, state.StateJ1HatchDesc)
		b.sendWithKeyboard(ctx, chatID, "📝 Описание люкового хозяйства (необязательно):", func(kb *maxbot.Keyboard) {
			skipButton(kb.AddRow(), "hatch_desc")
		})
	case "hatch_desc":
		state.SetState(chatID, state.StateJ1Year)
		b.sendText(ctx, chatID, "🔢 Введите год ввода трубопровода в эксплуатацию:")
	case "access_photo":
		b.afterJ2AccessPhoto(ctx, chatID)
	case "access_desc":
		b.afterJ2AccessDesc(ctx, chatID)
	case "gas_desc":
		b.afterJ2GasDesc(ctx, chatID)
	case "water_desc":
		b.afterJ2WaterDesc(ctx, chatID)
	case "drainage_photo":
		b.afterJ2DrainagePhoto(ctx, chatID)
	case "drainage_desc":
		b.afterJ2DrainageDesc(ctx, chatID)
	case "ventilation_desc":
		b.afterJ2VentilationDesc(ctx, chatID)
	case "silting_desc":
		b.afterJ2SiltingDesc(ctx, chatID)
	case "drain_desc":
		b.startSection4(ctx, chatID)
	case "equip_desc":
		b.showSaveConfirm(ctx, chatID)
	case "insulation_photo":
		b.afterJ4InsulationPhoto(ctx, chatID)
	case "metal_r1_photo":
		b.afterJ4MetalR1Photo(ctx, chatID)
	case "metal_r2_photo":
		b.afterJ4MetalR2Photo(ctx, chatID)
	case "valve_r1_photo":
		b.afterJ4ValveR1Photo(ctx, chatID)
	case "valve_r2_photo":
		b.afterJ4ValveR2Photo(ctx, chatID)
	default:
		if strings.HasSuffix(skipKey, "_photo") || state.GetDataString(chatID, "j3_photo_key") != "" {
			if key := state.GetDataString(chatID, "j3_photo_key"); key != "" {
				b.afterJ3Photo(ctx, chatID)
			}
		}
	}
}

func (b *Bot) tryHandlePhoto(ctx context.Context, chatID int64, upd *schemes.MessageCreatedUpdate) bool {
	photoID := extractPhotoID(upd)
	if photoID == "" {
		return false
	}
	st := state.GetCurrentState(chatID)
	key := photoKeyForState(st, chatID)
	if key == "" {
		return false
	}
	state.SetData(chatID, key, photoID)
	b.advanceAfterPhoto(ctx, chatID, st)
	return true
}

func extractPhotoID(upd *schemes.MessageCreatedUpdate) string {
	for _, att := range upd.Message.Body.Attachments {
		if p, ok := att.(*schemes.PhotoAttachment); ok {
			return strconv.FormatInt(p.Payload.PhotoId, 10)
		}
	}
	return ""
}

func photoKeyForState(st string, chatID int64) string {
	m := map[string]string{
		state.StateJ1ZonesPhoto:        "protection_zones_photo",
		state.StateJ2AccessPhoto:       "access_photo",
		state.StateJ2DrainagePhoto:     "drainage_photo",
		state.StateJ3ConcreteCorrPhoto: "concrete_damaged_photo",
		state.StateJ3ConcreteOkPhoto:   "concrete_ok_photo",
		state.StateJ3SupportsCorrPhoto: "supports_corrosion_photo",
		state.StateJ3SupportsOkPhoto:   "supports_ok_photo",
		state.StateJ3PipeR1CorrPhoto:   "pipe_r1_corrosion_photo",
		state.StateJ3PipeR1OkPhoto:     "pipe_r1_ok_photo",
		state.StateJ3PipeR2CorrPhoto:   "pipe_r2_corrosion_photo",
		state.StateJ3PipeR2OkPhoto:     "pipe_r2_ok_photo",
		state.StateJ3ValvePhoto:        "valve_main_photo",
		state.StateJ3DrainPhoto:        "drain_line_photo",
		state.StateJ4InsulationPhoto:   "insulation_photo",
		state.StateJ4MetalR1Photo:      "metal_r1_photo",
		state.StateJ4MetalR2Photo:      "metal_r2_photo",
		state.StateJ4ValveR1Photo:      "valve_r1_photo",
		state.StateJ4ValveR2Photo:      "valve_r2_photo",
	}
	if k, ok := m[st]; ok {
		return k
	}
	if k := state.GetDataString(chatID, "j3_photo_key"); k != "" {
		return k
	}
	if k := state.GetDataString(chatID, "j4_photo_key"); k != "" {
		return k
	}
	return ""
}

func (b *Bot) advanceAfterPhoto(ctx context.Context, chatID int64, st string) {
	switch st {
	case state.StateJ1ZonesPhoto:
		state.SetState(chatID, state.StateJ1HatchDesc)
		b.sendWithKeyboard(ctx, chatID, "📝 Описание люкового хозяйства (необязательно):", func(kb *maxbot.Keyboard) {
			skipButton(kb.AddRow(), "hatch_desc")
		})
	case state.StateJ2AccessPhoto:
		b.afterJ2AccessPhoto(ctx, chatID)
	case state.StateJ2DrainagePhoto:
		b.afterJ2DrainagePhoto(ctx, chatID)
	case state.StateJ3ConcreteCorrPhoto, state.StateJ3ConcreteOkPhoto, state.StateJ3SupportsCorrPhoto,
		state.StateJ3SupportsOkPhoto, state.StateJ3PipeR1CorrPhoto, state.StateJ3PipeR1OkPhoto,
		state.StateJ3PipeR2CorrPhoto, state.StateJ3PipeR2OkPhoto, state.StateJ3ValvePhoto, state.StateJ3DrainPhoto:
		b.afterJ3Photo(ctx, chatID)
	case state.StateJ4InsulationPhoto:
		b.afterJ4InsulationPhoto(ctx, chatID)
	case state.StateJ4MetalR1Photo:
		b.afterJ4MetalR1Photo(ctx, chatID)
	case state.StateJ4MetalR2Photo:
		b.afterJ4MetalR2Photo(ctx, chatID)
	case state.StateJ4ValveR1Photo:
		b.afterJ4ValveR1Photo(ctx, chatID)
	case state.StateJ4ValveR2Photo:
		b.afterJ4ValveR2Photo(ctx, chatID)
	}
}