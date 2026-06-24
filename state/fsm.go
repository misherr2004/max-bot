package state

import "sync"

const (
	StateStart            = "start"
	StateWaitPhone        = "wait_phone"
	StateWaitPhoneApproval = "wait_phone_approval"
	StateRegName          = "reg_name"
	StateRegPatronymic    = "reg_patronymic"
	StateRegLastName      = "reg_last_name"
	StateRegPosition      = "reg_position"
	StateRegExperience    = "reg_experience"
	StateRegWorkArea      = "reg_work_area"
	StateMainMenu         = "main_menu"
	StateProfile          = "profile"
	StateProfileEdit      = "profile_edit"
	StateJ1Date           = "j1_date"
	StateJ1Time           = "j1_time"
	StateJ1Address        = "j1_address"
	StateJ1Diameter       = "j1_diameter"
	StateJ1Zones          = "j1_zones"
	StateJ1ZonesPhoto     = "j1_zones_photo"
	StateJ1HatchDesc      = "j1_hatch_desc"
	StateJ1Year           = "j1_year"
	StateJ2Access         = "j2_access"
	StateJ2AccessPhoto    = "j2_access_photo"
	StateJ2AccessDesc     = "j2_access_desc"
	StateJ2Gas            = "j2_gas"
	StateJ2GasDesc        = "j2_gas_desc"
	StateJ2WaterTemp      = "j2_water_temp"
	StateJ2WaterSource    = "j2_water_source"
	StateJ2WaterDesc      = "j2_water_desc"
	StateJ2Drainage       = "j2_drainage"
	StateJ2DrainagePhoto  = "j2_drainage_photo"
	StateJ2DrainageDesc   = "j2_drainage_desc"
	StateJ2Ventilation    = "j2_ventilation"
	StateJ2VentilationDesc = "j2_ventilation_desc"
	StateJ2Silting        = "j2_silting"
	StateJ2SiltingDesc    = "j2_silting_desc"
	StateJ3ConcreteCorr       = "j3_concrete_corr"
	StateJ3ConcreteCorrPhoto  = "j3_concrete_corr_photo"
	StateJ3ConcreteOk         = "j3_concrete_ok"
	StateJ3ConcreteOkPhoto    = "j3_concrete_ok_photo"
	StateJ3SupportsCorr       = "j3_supports_corr"
	StateJ3SupportsCorrPhoto  = "j3_supports_corr_photo"
	StateJ3SupportsOk         = "j3_supports_ok"
	StateJ3SupportsOkPhoto    = "j3_supports_ok_photo"
	StateJ3PipeR1Corr           = "j3_pipe_r1_corr"
	StateJ3PipeR1CorrPhoto      = "j3_pipe_r1_corr_photo"
	StateJ3PipeR1Ok             = "j3_pipe_r1_ok"
	StateJ3PipeR1OkPhoto        = "j3_pipe_r1_ok_photo"
	StateJ3PipeR2Corr           = "j3_pipe_r2_corr"
	StateJ3PipeR2CorrPhoto      = "j3_pipe_r2_corr_photo"
	StateJ3PipeR2Ok             = "j3_pipe_r2_ok"
	StateJ3PipeR2OkPhoto        = "j3_pipe_r2_ok_photo"
	StateJ3Valve                 = "j3_valve"
	StateJ3ValvePhoto            = "j3_valve_photo"
	StateJ3Drain                 = "j3_drain"
	StateJ3DrainPhoto            = "j3_drain_photo"
	StateJ3DrainDesc             = "j3_drain_desc"
	StateJ4Diameter              = "j4_diameter"
	StateJ4Insulation            = "j4_insulation"
	StateJ4InsulationPhoto       = "j4_insulation_photo"
	StateJ4MetalR1               = "j4_metal_r1"
	StateJ4MetalR1Photo          = "j4_metal_r1_photo"
	StateJ4MetalR2               = "j4_metal_r2"
	StateJ4MetalR2Photo          = "j4_metal_r2_photo"
	StateJ4ValveR1               = "j4_valve_r1"
	StateJ4ValveR1Photo          = "j4_valve_r1_photo"
	StateJ4ValveR2               = "j4_valve_r2"
	StateJ4ValveR2Photo          = "j4_valve_r2_photo"
	StateJ4EquipDesc             = "j4_equip_desc"
	StateJSaveConfirm            = "j_save_confirm"
	StateMeasures                = "measures"
	StateMeasuresSave            = "measures_save"
	StateExperts                 = "experts"
	StateExpertChatMsg           = "expert_chat_msg"
	StateAdminMenu               = "admin_menu"
	StateAdminUpload             = "admin_upload"
	StateAdminAddExpert          = "admin_add_expert"
	StateAdminRemoveExpert       = "admin_remove_expert"
)

type UserState struct {
	State string
	Data  map[string]interface{}
	mu    sync.Mutex
}

var userStates = struct {
	sync.RWMutex
	m map[int64]*UserState
}{m: make(map[int64]*UserState)}

func GetState(chatID int64) *UserState {
	userStates.RLock()
	s, ok := userStates.m[chatID]
	userStates.RUnlock()
	if ok {
		return s
	}
	userStates.Lock()
	defer userStates.Unlock()
	if s, ok = userStates.m[chatID]; ok {
		return s
	}
	s = &UserState{State: StateStart, Data: make(map[string]interface{})}
	userStates.m[chatID] = s
	return s
}

func SetState(chatID int64, state string) {
	s := GetState(chatID)
	s.mu.Lock()
	s.State = state
	s.mu.Unlock()
}

func GetCurrentState(chatID int64) string {
	s := GetState(chatID)
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.State
}

func SetData(chatID int64, key string, value interface{}) {
	s := GetState(chatID)
	s.mu.Lock()
	s.Data[key] = value
	s.mu.Unlock()
}

func GetData(chatID int64, key string) interface{} {
	s := GetState(chatID)
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Data[key]
}

func GetDataString(chatID int64, key string) string {
	v := GetData(chatID, key)
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func ClearSession(chatID int64) {
	userStates.Lock()
	delete(userStates.m, chatID)
	userStates.Unlock()
}
