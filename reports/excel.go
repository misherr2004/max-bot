package reports

import (
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"

	"kontroler-ts/db"
)

func ExportUsers(users []*db.User, path string) error {
	f := excelize.NewFile()
	sheet := "Пользователи"
	f.SetSheetName("Sheet1", sheet)
	headers := []string{"№", "Телефон", "ФИО", "Должность", "Стаж", "Участок"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}
	for i, u := range users {
		row := i + 2
		f.SetCellValue(sheet, fmt.Sprintf("A%d", row), i+1)
		f.SetCellValue(sheet, fmt.Sprintf("B%d", row), u.Phone)
		f.SetCellValue(sheet, fmt.Sprintf("C%d", row), u.FullName())
		f.SetCellValue(sheet, fmt.Sprintf("D%d", row), u.Position)
		f.SetCellValue(sheet, fmt.Sprintf("E%d", row), u.Experience)
		f.SetCellValue(sheet, fmt.Sprintf("F%d", row), u.WorkArea)
	}
	return f.SaveAs(path)
}

func ExportMeasures(items []struct {
	Measure  db.Measure
	Journal  db.Journal
	User     db.User
	Issues   string
	Comments string
}, path string) error {
	f := excelize.NewFile()
	sheet := "Мероприятия"
	f.SetSheetName("Sheet1", sheet)
	headers := []string{"№", "Раздел с нарушениями", "Дата", "Список нарушений", "Фото", "Комментарии", "ФИО", "Телефон"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}
	for i, item := range items {
		row := i + 2
		photos := collectPhotos(&item.Journal)
		f.SetCellValue(sheet, fmt.Sprintf("A%d", row), i+1)
		f.SetCellValue(sheet, fmt.Sprintf("B%d", row), violationSection(&item.Journal))
		f.SetCellValue(sheet, fmt.Sprintf("C%d", row), item.Journal.InspectionDate)
		f.SetCellValue(sheet, fmt.Sprintf("D%d", row), item.Issues)
		f.SetCellValue(sheet, fmt.Sprintf("E%d", row), photos)
		f.SetCellValue(sheet, fmt.Sprintf("F%d", row), item.Comments)
		f.SetCellValue(sheet, fmt.Sprintf("G%d", row), item.User.FullName())
		f.SetCellValue(sheet, fmt.Sprintf("H%d", row), item.User.Phone)
	}
	return f.SaveAs(path)
}

func violationSection(j *db.Journal) string {
	sections := []string{}
	if j.ProtectionZonesViolated == 1 || j.MainDiameter != "" {
		sections = append(sections, "1")
	}
	if j.AccessIssue != "" {
		sections = append(sections, "2")
	}
	if j.ConcreteDamaged == 1 || j.ValveMain != "" {
		sections = append(sections, "3")
	}
	if j.BranchDiameter != "" || j.InsulationState != "" {
		sections = append(sections, "4")
	}
	return strings.Join(sections, ", ")
}

func collectPhotos(j *db.Journal) string {
	var photos []string
	for _, p := range []string{
		j.ProtectionZonesPhoto, j.AccessPhoto, j.DrainagePhoto,
		j.ConcreteDamagedPhoto, j.ConcreteOkPhoto, j.SupportsCorrosionPhoto,
		j.SupportsOkPhoto, j.PipeR1CorrosionPhoto, j.PipeR1OkPhoto,
		j.PipeR2CorrosionPhoto, j.PipeR2OkPhoto, j.ValveMainPhoto,
		j.DrainLinePhoto, j.InsulationPhoto, j.MetalR1Photo, j.MetalR2Photo,
		j.ValveR1Photo, j.ValveR2Photo,
	} {
		if strings.TrimSpace(p) != "" {
			photos = append(photos, p)
		}
	}
	return strings.Join(photos, ", ")
}

func ImportLists(database *db.DB, path string) error {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return err
	}
	defer f.Close()

	if phones, err := readColumn(f, "Сотрудники", 1); err == nil && len(phones) > 0 {
		_ = database.ClearAndInsertPhones(phones)
	}
	if addresses, err := readColumn(f, "Адреса ТК", 1); err == nil && len(addresses) > 0 {
		_ = database.ClearAndInsertTKAddresses(addresses)
	}
	if rows, err := f.GetRows("Эксперты"); err == nil && len(rows) > 1 {
		for _, row := range rows[1:] {
			if len(row) >= 2 {
				phone := normalizePhoneImport(row[1])
				_ = database.AddAllowedPhone(phone)
				u, err := database.GetUserByPhone(phone)
				if err == nil {
					_ = database.UpsertExpertByPhone(phone)
					_ = u
				}
			}
		}
	}
	return nil
}

func readColumn(f *excelize.File, sheet string, col int) ([]string, error) {
	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, err
	}
	var result []string
	for i, row := range rows {
		if i == 0 {
			continue
		}
		if col-1 < len(row) && strings.TrimSpace(row[col-1]) != "" {
			result = append(result, strings.TrimSpace(row[col-1]))
		}
	}
	return result, nil
}

func normalizePhoneImport(s string) string {
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "-", "")
	if strings.HasPrefix(s, "8") && len(s) == 11 {
		s = "+7" + s[1:]
	}
	if strings.HasPrefix(s, "7") && len(s) == 11 && !strings.HasPrefix(s, "+") {
		s = "+" + s
	}
	return s
}
