//go:build !windows || !386

package handler

import (
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"sort"

	"github.com/xuri/excelize/v2"
)

func writeExcelResource(w http.ResponseWriter, resource string, rows any) {
	encoded, err := json.Marshal(rows)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var data []map[string]any
	if err := json.Unmarshal(encoded, &data); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	book := excelize.NewFile()
	defer book.Close()
	sheet := "数据"
	book.SetSheetName("Sheet1", sheet)
	if len(data) > 0 {
		headers := make([]string, 0, len(data[0]))
		for key := range data[0] {
			if key != "created_at" && key != "updated_at" {
				headers = append(headers, key)
			}
		}
		sort.Strings(headers)
		for index, key := range headers {
			cell, _ := excelize.CoordinatesToCellName(index+1, 1)
			_ = book.SetCellValue(sheet, cell, key)
		}
		for rowIndex, row := range data {
			for columnIndex, key := range headers {
				cell, _ := excelize.CoordinatesToCellName(columnIndex+1, rowIndex+2)
				_ = book.SetCellValue(sheet, cell, row[key])
			}
		}
		lastCell, _ := excelize.CoordinatesToCellName(len(headers), 1)
		style, _ := book.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Color: "FFFFFF"}, Fill: excelize.Fill{Type: "pattern", Color: []string{"167052"}, Pattern: 1}})
		_ = book.SetCellStyle(sheet, "A1", lastCell, style)
		_ = book.SetPanes(sheet, &excelize.Panes{Freeze: true, YSplit: 1})
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", `attachment; filename="`+resource+`.xlsx"`)
	_ = book.Write(w)
}

func readExcelResource(file multipart.File, spec resourceSpec) (any, error) {
	book, err := excelize.OpenReader(file)
	if err != nil {
		return nil, err
	}
	defer book.Close()
	sheets := book.GetSheetList()
	if len(sheets) == 0 {
		return nil, errors.New("Excel没有工作表")
	}
	rows, err := book.GetRows(sheets[0])
	if err != nil {
		return nil, err
	}
	return rowsFromCells(spec, rows)
}
