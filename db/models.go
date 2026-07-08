package db

import "time"

type User struct {
	ID           int64
	MaxUserID    string
	Phone        string
	FirstName    string
	LastName     string
	Patronymic   string
	Position     string
	Experience   string
	WorkArea     string
	Role         string
	RegisteredAt time.Time
	LastLogin    *time.Time
}

func (u *User) FullName() string {
	return u.LastName + " " + u.FirstName + " " + u.Patronymic
}

func (u *User) ShortName() string {
	return u.FirstName + " " + u.LastName
}

type Journal struct {
	ID        int64
	UserID    int64
	CreatedAt time.Time
	Status    string

	InspectionDate string
	InspectionTime string
	TkAddress      string
	MainDiameter   string

	ProtectionZonesViolated int
	ProtectionZonesPhoto    string
	HatchDescription        string
	PipelineYear            int
	ServiceLife             int
	ServiceGroup            string

	AccessIssue        string
	AccessPhoto        string
	AccessDescription  string
	GasHazard          int
	GasHazardDesc      string
	WaterTemp          string
	WaterSource        string
	WaterDescription   string
	Drainage           string
	DrainagePhoto      string
	DrainageDesc       string
	Ventilation        string
	VentilationDesc    string
	SiltingLevel       string
	SiltingDesc        string

	ConcreteDamaged      int
	ConcreteDamagedPhoto string
	ConcreteOk           int
	ConcreteOkPhoto      string
	SupportsCorrosion    string
	SupportsCorrosionPhoto string
	SupportsOk           int
	SupportsOkPhoto      string
	PipeR1Corrosion      string
	PipeR1CorrosionPhoto string
	PipeR1Ok             int
	PipeR1OkPhoto        string
	PipeR2Corrosion      string
	PipeR2CorrosionPhoto string
	PipeR2Ok             int
	PipeR2OkPhoto        string
	ValveMain            string
	ValveMainPhoto       string
	DrainLine            string
	DrainLinePhoto       string
	DrainLineDesc        string

	BranchDiameter      string
	InsulationState     string
	InsulationPhoto     string
	MetalR1             string
	MetalR1Photo        string
	MetalR2             string
	MetalR2Photo        string
	ValveR1             string
	ValveR1Photo        string
	ValveR2             string
	ValveR2Photo        string
	EquipmentDesc       string
}

type Measure struct {
	ID        int64
	JournalID int64
	UserID    int64
	CreatedAt time.Time
	Status    string
}

type Expert struct {
	ID       int64
	UserID   int64
	IsActive bool
	User     *User
}

type TKAddress struct {
	ID      int64
	Address string
}
