package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	conn *sql.DB
}

func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite3", path+"?_foreign_keys=on")
	if err != nil {
		return nil, err
	}
	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, err
	}
	return db, nil
}

func (db *DB) Close() error {
	return db.conn.Close()
}

func (db *DB) migrate() error {
	schema := `
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    max_user_id TEXT UNIQUE NOT NULL,
    phone TEXT,
    first_name TEXT,
    last_name TEXT,
    patronymic TEXT,
    position TEXT,
    experience TEXT,
    work_area TEXT,
    role TEXT DEFAULT 'user',
    registered_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_login DATETIME
);
CREATE TABLE IF NOT EXISTS allowed_phones (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    phone TEXT UNIQUE NOT NULL
);
CREATE TABLE IF NOT EXISTS tk_addresses (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    address TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS experts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER REFERENCES users(id),
    is_active INTEGER DEFAULT 1
);
CREATE TABLE IF NOT EXISTS journals (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER REFERENCES users(id),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    status TEXT DEFAULT 'draft',
    inspection_date TEXT, inspection_time TEXT, tk_address TEXT, main_diameter TEXT,
    protection_zones_violated INTEGER, protection_zones_photo TEXT, hatch_description TEXT,
    pipeline_year INTEGER, service_life INTEGER, service_group TEXT,
    access_issue TEXT, access_photo TEXT, access_description TEXT,
    gas_hazard INTEGER, gas_hazard_description TEXT,
    water_temp TEXT, water_source TEXT, water_description TEXT,
    drainage TEXT, drainage_photo TEXT, drainage_description TEXT,
    ventilation TEXT, ventilation_description TEXT,
    silting_level TEXT, silting_description TEXT,
    concrete_damaged INTEGER, concrete_damaged_photo TEXT,
    concrete_ok INTEGER, concrete_ok_photo TEXT,
    supports_corrosion TEXT, supports_corrosion_photo TEXT,
    supports_ok INTEGER, supports_ok_photo TEXT,
    pipe_r1_corrosion TEXT, pipe_r1_corrosion_photo TEXT,
    pipe_r1_ok INTEGER, pipe_r1_ok_photo TEXT,
    pipe_r2_corrosion TEXT, pipe_r2_corrosion_photo TEXT,
    pipe_r2_ok INTEGER, pipe_r2_ok_photo TEXT,
    valve_main TEXT, valve_main_photo TEXT,
    drain_line TEXT, drain_line_photo TEXT, drain_line_description TEXT,
    branch_diameter TEXT, insulation_state TEXT, insulation_photo TEXT,
    metal_r1 TEXT, metal_r1_photo TEXT, metal_r2 TEXT, metal_r2_photo TEXT,
    valve_r1 TEXT, valve_r1_photo TEXT, valve_r2 TEXT, valve_r2_photo TEXT,
    equipment_description TEXT
);
CREATE TABLE IF NOT EXISTS measures (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    journal_id INTEGER REFERENCES journals(id),
    user_id INTEGER REFERENCES users(id),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    status TEXT DEFAULT 'active'
);
CREATE TABLE IF NOT EXISTS messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    from_user_id INTEGER REFERENCES users(id),
    to_user_id INTEGER REFERENCES users(id),
    text TEXT,
    sent_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    read INTEGER DEFAULT 0
);`
	if _, err := db.conn.Exec(schema); err != nil {
		return err
	}
	return db.seedTKAddresses()
}

func (db *DB) seedTKAddresses() error {
	var count int
	if err := db.conn.QueryRow("SELECT COUNT(*) FROM tk_addresses").Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	addresses := []string{
		"УТ-1 Сертолово, ул. Пограничная д.4/2",
		"УТ-2 Сертолово, ул. Пограничная д.4/2",
		"УТ-3 Сертолово, ул. Пограничная д.4/2",
		"УТ-4 Сертолово, ул. Пограничная д.4/2",
		"УТ-5 Сертолово, ул. Пограничная д.4/1",
		"УТ-6 Сертолово, ул. Пограничная д.4/3",
		"УТ-7 Сертолово, ул. Центральная д.14/4",
		"УТ-8 Сертолово, ул. Центральная д.14/4",
	}
	for _, a := range addresses {
		if _, err := db.conn.Exec("INSERT INTO tk_addresses (address) VALUES (?)", a); err != nil {
			return err
		}
	}
	return nil
}

func scanUser(row interface{ Scan(...any) error }) (*User, error) {
	u := &User{}
	var lastLogin sql.NullTime
	err := row.Scan(
		&u.ID, &u.MaxUserID, &u.Phone, &u.FirstName, &u.LastName, &u.Patronymic,
		&u.Position, &u.Experience, &u.WorkArea, &u.Role, &u.RegisteredAt, &lastLogin,
	)
	if err != nil {
		return nil, err
	}
	if lastLogin.Valid {
		u.LastLogin = &lastLogin.Time
	}
	return u, nil
}

const userCols = `id, max_user_id, phone, first_name, last_name, patronymic, position, experience, work_area, role, registered_at, last_login`

func (db *DB) GetUserByMaxID(maxUserID string) (*User, error) {
	row := db.conn.QueryRow("SELECT "+userCols+" FROM users WHERE max_user_id = ?", maxUserID)
	return scanUser(row)
}

func (db *DB) GetUserByID(id int64) (*User, error) {
	row := db.conn.QueryRow("SELECT "+userCols+" FROM users WHERE id = ?", id)
	return scanUser(row)
}

func (db *DB) GetUserByPhone(phone string) (*User, error) {
	row := db.conn.QueryRow("SELECT "+userCols+" FROM users WHERE phone = ?", phone)
	return scanUser(row)
}

func (db *DB) IsPhoneAllowed(phone string) (bool, error) {
	var n int
	err := db.conn.QueryRow("SELECT COUNT(*) FROM allowed_phones WHERE phone = ?", phone).Scan(&n)
	return n > 0, err
}

func (db *DB) AddAllowedPhone(phone string) error {
	_, err := db.conn.Exec("INSERT OR IGNORE INTO allowed_phones (phone) VALUES (?)", phone)
	return err
}

func (db *DB) UpdateLastLogin(userID int64) error {
	_, err := db.conn.Exec("UPDATE users SET last_login = ? WHERE id = ?", time.Now(), userID)
	return err
}

func (db *DB) CreateUser(maxUserID, phone string) (*User, error) {
	res, err := db.conn.Exec(
		"INSERT INTO users (max_user_id, phone) VALUES (?, ?)",
		maxUserID, phone,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return db.GetUserByID(id)
}

func (db *DB) UpdateUserProfile(userID int64, first, patronymic, last, position, experience, workArea string) error {
	_, err := db.conn.Exec(
		`UPDATE users SET first_name=?, patronymic=?, last_name=?, position=?, experience=?, work_area=? WHERE id=?`,
		first, patronymic, last, position, experience, workArea, userID,
	)
	return err
}

func (db *DB) GetAdmins() ([]*User, error) {
	rows, err := db.conn.Query("SELECT " + userCols + " FROM users WHERE role = 'admin'")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []*User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (db *DB) GetAllUsers() ([]*User, error) {
	rows, err := db.conn.Query("SELECT " + userCols + " FROM users ORDER BY last_name, first_name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []*User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (db *DB) CountJournals(userID int64) (int, error) {
	var n int
	err := db.conn.QueryRow("SELECT COUNT(*) FROM journals WHERE user_id = ? AND status = 'saved'", userID).Scan(&n)
	return n, err
}

func (db *DB) CountMeasures(userID int64) (int, error) {
	var n int
	err := db.conn.QueryRow("SELECT COUNT(*) FROM measures WHERE user_id = ?", userID).Scan(&n)
	return n, err
}

func (db *DB) GetTKAddresses() ([]TKAddress, error) {
	rows, err := db.conn.Query("SELECT id, address FROM tk_addresses ORDER BY address")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []TKAddress
	for rows.Next() {
		var a TKAddress
		if err := rows.Scan(&a.ID, &a.Address); err != nil {
			return nil, err
		}
		list = append(list, a)
	}
	return list, rows.Err()
}

func (db *DB) ClearAndInsertPhones(phones []string) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM allowed_phones"); err != nil {
		tx.Rollback()
		return err
	}
	for _, p := range phones {
		if _, err := tx.Exec("INSERT INTO allowed_phones (phone) VALUES (?)", p); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (db *DB) ClearAndInsertTKAddresses(addresses []string) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM tk_addresses"); err != nil {
		tx.Rollback()
		return err
	}
	for _, a := range addresses {
		if _, err := tx.Exec("INSERT INTO tk_addresses (address) VALUES (?)", a); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (db *DB) SaveJournal(j *Journal) (int64, error) {
	if j.ID > 0 {
		return j.ID, db.updateJournal(j)
	}
	res, err := db.conn.Exec(`
INSERT INTO journals (
    user_id, status, inspection_date, inspection_time, tk_address, main_diameter,
    protection_zones_violated, protection_zones_photo, hatch_description,
    pipeline_year, service_life, service_group,
    access_issue, access_photo, access_description, gas_hazard, gas_hazard_description,
    water_temp, water_source, water_description, drainage, drainage_photo, drainage_description,
    ventilation, ventilation_description, silting_level, silting_description,
    concrete_damaged, concrete_damaged_photo, concrete_ok, concrete_ok_photo,
    supports_corrosion, supports_corrosion_photo, supports_ok, supports_ok_photo,
    pipe_r1_corrosion, pipe_r1_corrosion_photo, pipe_r1_ok, pipe_r1_ok_photo,
    pipe_r2_corrosion, pipe_r2_corrosion_photo, pipe_r2_ok, pipe_r2_ok_photo,
    valve_main, valve_main_photo, drain_line, drain_line_photo, drain_line_description,
    branch_diameter, insulation_state, insulation_photo,
    metal_r1, metal_r1_photo, metal_r2, metal_r2_photo,
    valve_r1, valve_r1_photo, valve_r2, valve_r2_photo, equipment_description
) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		j.UserID, j.Status, j.InspectionDate, j.InspectionTime, j.TkAddress, j.MainDiameter,
		j.ProtectionZonesViolated, j.ProtectionZonesPhoto, j.HatchDescription,
		j.PipelineYear, j.ServiceLife, j.ServiceGroup,
		j.AccessIssue, j.AccessPhoto, j.AccessDescription, j.GasHazard, j.GasHazardDesc,
		j.WaterTemp, j.WaterSource, j.WaterDescription, j.Drainage, j.DrainagePhoto, j.DrainageDesc,
		j.Ventilation, j.VentilationDesc, j.SiltingLevel, j.SiltingDesc,
		j.ConcreteDamaged, j.ConcreteDamagedPhoto, j.ConcreteOk, j.ConcreteOkPhoto,
		j.SupportsCorrosion, j.SupportsCorrosionPhoto, j.SupportsOk, j.SupportsOkPhoto,
		j.PipeR1Corrosion, j.PipeR1CorrosionPhoto, j.PipeR1Ok, j.PipeR1OkPhoto,
		j.PipeR2Corrosion, j.PipeR2CorrosionPhoto, j.PipeR2Ok, j.PipeR2OkPhoto,
		j.ValveMain, j.ValveMainPhoto, j.DrainLine, j.DrainLinePhoto, j.DrainLineDesc,
		j.BranchDiameter, j.InsulationState, j.InsulationPhoto,
		j.MetalR1, j.MetalR1Photo, j.MetalR2, j.MetalR2Photo,
		j.ValveR1, j.ValveR1Photo, j.ValveR2, j.ValveR2Photo, j.EquipmentDesc,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (db *DB) updateJournal(j *Journal) error {
	_, err := db.conn.Exec(`
UPDATE journals SET status=?, inspection_date=?, inspection_time=?, tk_address=?, main_diameter=?,
    protection_zones_violated=?, protection_zones_photo=?, hatch_description=?,
    pipeline_year=?, service_life=?, service_group=?,
    access_issue=?, access_photo=?, access_description=?, gas_hazard=?, gas_hazard_description=?,
    water_temp=?, water_source=?, water_description=?, drainage=?, drainage_photo=?, drainage_description=?,
    ventilation=?, ventilation_description=?, silting_level=?, silting_description=?,
    concrete_damaged=?, concrete_damaged_photo=?, concrete_ok=?, concrete_ok_photo=?,
    supports_corrosion=?, supports_corrosion_photo=?, supports_ok=?, supports_ok_photo=?,
    pipe_r1_corrosion=?, pipe_r1_corrosion_photo=?, pipe_r1_ok=?, pipe_r1_ok_photo=?,
    pipe_r2_corrosion=?, pipe_r2_corrosion_photo=?, pipe_r2_ok=?, pipe_r2_ok_photo=?,
    valve_main=?, valve_main_photo=?, drain_line=?, drain_line_photo=?, drain_line_description=?,
    branch_diameter=?, insulation_state=?, insulation_photo=?,
    metal_r1=?, metal_r1_photo=?, metal_r2=?, metal_r2_photo=?,
    valve_r1=?, valve_r1_photo=?, valve_r2=?, valve_r2_photo=?, equipment_description=?
WHERE id=?`,
		j.Status, j.InspectionDate, j.InspectionTime, j.TkAddress, j.MainDiameter,
		j.ProtectionZonesViolated, j.ProtectionZonesPhoto, j.HatchDescription,
		j.PipelineYear, j.ServiceLife, j.ServiceGroup,
		j.AccessIssue, j.AccessPhoto, j.AccessDescription, j.GasHazard, j.GasHazardDesc,
		j.WaterTemp, j.WaterSource, j.WaterDescription, j.Drainage, j.DrainagePhoto, j.DrainageDesc,
		j.Ventilation, j.VentilationDesc, j.SiltingLevel, j.SiltingDesc,
		j.ConcreteDamaged, j.ConcreteDamagedPhoto, j.ConcreteOk, j.ConcreteOkPhoto,
		j.SupportsCorrosion, j.SupportsCorrosionPhoto, j.SupportsOk, j.SupportsOkPhoto,
		j.PipeR1Corrosion, j.PipeR1CorrosionPhoto, j.PipeR1Ok, j.PipeR1OkPhoto,
		j.PipeR2Corrosion, j.PipeR2CorrosionPhoto, j.PipeR2Ok, j.PipeR2OkPhoto,
		j.ValveMain, j.ValveMainPhoto, j.DrainLine, j.DrainLinePhoto, j.DrainLineDesc,
		j.BranchDiameter, j.InsulationState, j.InsulationPhoto,
		j.MetalR1, j.MetalR1Photo, j.MetalR2, j.MetalR2Photo,
		j.ValveR1, j.ValveR1Photo, j.ValveR2, j.ValveR2Photo, j.EquipmentDesc,
		j.ID,
	)
	return err
}

func (db *DB) GetLastJournal(userID int64) (*Journal, error) {
	row := db.conn.QueryRow(`
SELECT id, user_id, created_at, status,
    inspection_date, inspection_time, tk_address, main_diameter,
    protection_zones_violated, protection_zones_photo, hatch_description,
    pipeline_year, service_life, service_group,
    access_issue, access_photo, access_description, gas_hazard, gas_hazard_description,
    water_temp, water_source, water_description, drainage, drainage_photo, drainage_description,
    ventilation, ventilation_description, silting_level, silting_description,
    concrete_damaged, concrete_damaged_photo, concrete_ok, concrete_ok_photo,
    supports_corrosion, supports_corrosion_photo, supports_ok, supports_ok_photo,
    pipe_r1_corrosion, pipe_r1_corrosion_photo, pipe_r1_ok, pipe_r1_ok_photo,
    pipe_r2_corrosion, pipe_r2_corrosion_photo, pipe_r2_ok, pipe_r2_ok_photo,
    valve_main, valve_main_photo, drain_line, drain_line_photo, drain_line_description,
    branch_diameter, insulation_state, insulation_photo,
    metal_r1, metal_r1_photo, metal_r2, metal_r2_photo,
    valve_r1, valve_r1_photo, valve_r2, valve_r2_photo, equipment_description
FROM journals WHERE user_id = ? ORDER BY id DESC LIMIT 1`, userID)
	return scanJournal(row)
}

func scanJournal(row interface{ Scan(...any) error }) (*Journal, error) {
	j := &Journal{}
	err := row.Scan(
		&j.ID, &j.UserID, &j.CreatedAt, &j.Status,
		&j.InspectionDate, &j.InspectionTime, &j.TkAddress, &j.MainDiameter,
		&j.ProtectionZonesViolated, &j.ProtectionZonesPhoto, &j.HatchDescription,
		&j.PipelineYear, &j.ServiceLife, &j.ServiceGroup,
		&j.AccessIssue, &j.AccessPhoto, &j.AccessDescription, &j.GasHazard, &j.GasHazardDesc,
		&j.WaterTemp, &j.WaterSource, &j.WaterDescription, &j.Drainage, &j.DrainagePhoto, &j.DrainageDesc,
		&j.Ventilation, &j.VentilationDesc, &j.SiltingLevel, &j.SiltingDesc,
		&j.ConcreteDamaged, &j.ConcreteDamagedPhoto, &j.ConcreteOk, &j.ConcreteOkPhoto,
		&j.SupportsCorrosion, &j.SupportsCorrosionPhoto, &j.SupportsOk, &j.SupportsOkPhoto,
		&j.PipeR1Corrosion, &j.PipeR1CorrosionPhoto, &j.PipeR1Ok, &j.PipeR1OkPhoto,
		&j.PipeR2Corrosion, &j.PipeR2CorrosionPhoto, &j.PipeR2Ok, &j.PipeR2OkPhoto,
		&j.ValveMain, &j.ValveMainPhoto, &j.DrainLine, &j.DrainLinePhoto, &j.DrainLineDesc,
		&j.BranchDiameter, &j.InsulationState, &j.InsulationPhoto,
		&j.MetalR1, &j.MetalR1Photo, &j.MetalR2, &j.MetalR2Photo,
		&j.ValveR1, &j.ValveR1Photo, &j.ValveR2, &j.ValveR2Photo, &j.EquipmentDesc,
	)
	if err != nil {
		return nil, err
	}
	return j, nil
}

func (db *DB) CreateMeasure(journalID, userID int64) (int64, error) {
	res, err := db.conn.Exec(
		"INSERT INTO measures (journal_id, user_id, status) VALUES (?, ?, 'active')",
		journalID, userID,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (db *DB) GetActiveExperts() ([]*Expert, error) {
	rows, err := db.conn.Query(`
SELECT e.id, e.user_id, e.is_active,
       u.id, u.max_user_id, u.phone, u.first_name, u.last_name, u.patronymic,
       u.position, u.experience, u.work_area, u.role, u.registered_at, u.last_login
FROM experts e
JOIN users u ON u.id = e.user_id
WHERE e.is_active = 1
ORDER BY u.last_name, u.first_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*Expert
	for rows.Next() {
		e := &Expert{User: &User{}}
		var lastLogin sql.NullTime
		var isActive int
		if err := rows.Scan(
			&e.ID, &e.UserID, &isActive,
			&e.User.ID, &e.User.MaxUserID, &e.User.Phone, &e.User.FirstName, &e.User.LastName,
			&e.User.Patronymic, &e.User.Position, &e.User.Experience, &e.User.WorkArea,
			&e.User.Role, &e.User.RegisteredAt, &lastLogin,
		); err != nil {
			return nil, err
		}
		e.IsActive = isActive == 1
		if lastLogin.Valid {
			e.User.LastLogin = &lastLogin.Time
		}
		list = append(list, e)
	}
	return list, rows.Err()
}

func (db *DB) CountActiveExperts() (int, error) {
	var n int
	err := db.conn.QueryRow("SELECT COUNT(*) FROM experts WHERE is_active = 1").Scan(&n)
	return n, err
}

func (db *DB) AddExpert(userID int64) error {
	_, err := db.conn.Exec("INSERT INTO experts (user_id, is_active) VALUES (?, 1)", userID)
	return err
}

func (db *DB) RemoveExpert(expertID int64) error {
	_, err := db.conn.Exec("UPDATE experts SET is_active = 0 WHERE id = ?", expertID)
	return err
}

func (db *DB) UpsertExpertByPhone(phone string) error {
	u, err := db.GetUserByPhone(phone)
	if err != nil {
		return fmt.Errorf("пользователь с телефоном %s не найден", phone)
	}
	var n int
	if err := db.conn.QueryRow("SELECT COUNT(*) FROM experts WHERE user_id = ?", u.ID).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		_, err = db.conn.Exec("UPDATE experts SET is_active = 1 WHERE user_id = ?", u.ID)
		return err
	}
	_, err = db.conn.Exec("INSERT INTO experts (user_id, is_active) VALUES (?, 1)", u.ID)
	return err
}

func (db *DB) SaveMessage(fromID, toID int64, text string) error {
	_, err := db.conn.Exec(
		"INSERT INTO messages (from_user_id, to_user_id, text) VALUES (?, ?, ?)",
		fromID, toID, text,
	)
	return err
}

func (db *DB) GetAllMeasuresWithDetails() ([]struct {
	Measure  Measure
	Journal  Journal
	User     User
	Issues   string
	Comments string
}, error) {
	rows, err := db.conn.Query(`
SELECT m.id, m.journal_id, m.user_id, m.created_at, m.status,
       j.id, j.user_id, j.created_at, j.status,
       j.inspection_date, j.tk_address, j.equipment_description,
       u.id, u.max_user_id, u.phone, u.first_name, u.last_name, u.patronymic,
       u.position, u.experience, u.work_area, u.role, u.registered_at, u.last_login
FROM measures m
JOIN journals j ON j.id = m.journal_id
JOIN users u ON u.id = m.user_id
ORDER BY m.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []struct {
		Measure  Measure
		Journal  Journal
		User     User
		Issues   string
		Comments string
	}
	for rows.Next() {
		var item struct {
			Measure  Measure
			Journal  Journal
			User     User
			Issues   string
			Comments string
		}
		var lastLogin sql.NullTime
		if err := rows.Scan(
			&item.Measure.ID, &item.Measure.JournalID, &item.Measure.UserID,
			&item.Measure.CreatedAt, &item.Measure.Status,
			&item.Journal.ID, &item.Journal.UserID, &item.Journal.CreatedAt, &item.Journal.Status,
			&item.Journal.InspectionDate, &item.Journal.TkAddress, &item.Journal.EquipmentDesc,
			&item.User.ID, &item.User.MaxUserID, &item.User.Phone, &item.User.FirstName,
			&item.User.LastName, &item.User.Patronymic, &item.User.Position, &item.User.Experience,
			&item.User.WorkArea, &item.User.Role, &item.User.RegisteredAt, &lastLogin,
		); err != nil {
			return nil, err
		}
		if lastLogin.Valid {
			item.User.LastLogin = &lastLogin.Time
		}
		full, _ := db.GetLastJournal(item.User.ID)
		if full != nil && full.ID == item.Journal.ID {
			item.Journal = *full
		}
		item.Issues = strings.Join(CollectJournalIssues(&item.Journal), "; ")
		item.Comments = CollectJournalComments(&item.Journal)
		result = append(result, item)
	}
	return result, rows.Err()
}

func CollectJournalIssues(j *Journal) []string {
	var issues []string
	add := func(label, val string, bad ...string) {
		for _, b := range bad {
			if val == b {
				issues = append(issues, label+": "+val)
				return
			}
		}
	}
	if j.ProtectionZonesViolated == 1 {
		issues = append(issues, "Охранные зоны нарушены")
	}
	add("Доступ", j.AccessIssue, "flooded", "steamed", "silted")
	add("Газоопасность", fmt.Sprintf("%d", j.GasHazard), "1")
	add("Вода", j.WaterTemp, "hot")
	add("Дренаж", j.Drainage, "clean", "repair", "notfound")
	add("Вентиляция", j.Ventilation, "ineffective", "clean")
	add("Заиливание", j.SiltingLevel, "touch", "full")
	if j.ConcreteDamaged == 1 {
		issues = append(issues, "Бетон разрушен")
	}
	if j.ConcreteOk == 0 && j.ConcreteDamaged == 0 && j.ConcreteOkPhoto != "" {
		issues = append(issues, "Конструкции не в хорошем состоянии")
	}
	add("Коррозия опор", j.SupportsCorrosion, "yes")
	if j.SupportsOk == 0 {
		issues = append(issues, "Опоры не в хорошем состоянии")
	}
	add("Коррозия Р1", j.PipeR1Corrosion, "yes", "insulation")
	if j.PipeR1Ok == 0 {
		issues = append(issues, "Металл Р1 не в хорошем состоянии")
	}
	add("Коррозия Р2", j.PipeR2Corrosion, "yes", "insulation")
	if j.PipeR2Ok == 0 {
		issues = append(issues, "Металл Р2 не в хорошем состоянии")
	}
	add("Запорная арматура", j.ValveMain, "gland", "replace")
	add("Спускная линия", j.DrainLine, "corrosion", "flooded")
	add("Изоляция", j.InsulationState, "partial", "missing")
	add("Металл Р1 ввод", j.MetalR1, "lt20", "gt20")
	add("Металл Р2 ввод", j.MetalR2, "lt20", "gt20")
	add("Арматура Р1", j.ValveR1, "gland", "replace")
	add("Арматура Р2", j.ValveR2, "gland", "replace")
	return issues
}

func CollectJournalComments(j *Journal) string {
	var parts []string
	for _, s := range []string{
		j.HatchDescription, j.AccessDescription, j.GasHazardDesc, j.WaterDescription,
		j.DrainageDesc, j.VentilationDesc, j.SiltingDesc, j.DrainLineDesc, j.EquipmentDesc,
	} {
		if strings.TrimSpace(s) != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, "\n")
}
