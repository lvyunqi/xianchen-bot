//go:build windows && 386

package handler

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"sort"
	"strconv"
	"strings"
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
	headers := []string{}
	if len(data) > 0 {
		for key := range data[0] {
			if key != "created_at" && key != "updated_at" {
				headers = append(headers, key)
			}
		}
		sort.Strings(headers)
	}
	var output bytes.Buffer
	archive := zip.NewWriter(&output)
	files := map[string]string{
		"[Content_Types].xml":        `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/></Types>`,
		"_rels/.rels":                `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`,
		"xl/workbook.xml":            `<?xml version="1.0" encoding="UTF-8"?><workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="数据" sheetId="1" r:id="rId1"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/></Relationships>`,
		"xl/worksheets/sheet1.xml":   buildSheetXML(headers, data),
	}
	for name, content := range files {
		entry, _ := archive.Create(name)
		_, _ = io.WriteString(entry, content)
	}
	_ = archive.Close()
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", `attachment; filename="`+resource+`.xlsx"`)
	_, _ = w.Write(output.Bytes())
}

func buildSheetXML(headers []string, data []map[string]any) string {
	var output strings.Builder
	output.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)
	rows := make([][]string, 0, len(data)+1)
	rows = append(rows, headers)
	for _, row := range data {
		values := make([]string, len(headers))
		for index, key := range headers {
			values[index] = fmt.Sprint(row[key])
		}
		rows = append(rows, values)
	}
	for rowIndex, row := range rows {
		fmt.Fprintf(&output, `<row r="%d">`, rowIndex+1)
		for columnIndex, value := range row {
			cell := columnName(columnIndex+1) + strconv.Itoa(rowIndex+1)
			output.WriteString(`<c r="` + cell + `" t="inlineStr"><is><t>`)
			_ = xml.EscapeText(&output, []byte(value))
			output.WriteString(`</t></is></c>`)
		}
		output.WriteString(`</row>`)
	}
	output.WriteString(`</sheetData></worksheet>`)
	return output.String()
}

func readExcelResource(file multipart.File, spec resourceSpec) (any, error) {
	data, err := io.ReadAll(io.LimitReader(file, 20<<20))
	if err != nil {
		return nil, err
	}
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	shared := []string{}
	if raw, found := zipFile(archive, "xl/sharedStrings.xml"); found {
		shared = parseSharedStrings(raw)
	}
	raw, found := zipFile(archive, "xl/worksheets/sheet1.xml")
	if !found {
		return nil, fmt.Errorf("Excel缺少第一个工作表")
	}
	rows, err := parseSheetRows(raw, shared)
	if err != nil {
		return nil, err
	}
	return rowsFromCells(spec, rows)
}

func zipFile(archive *zip.Reader, name string) ([]byte, bool) {
	for _, file := range archive.File {
		if file.Name != name {
			continue
		}
		reader, err := file.Open()
		if err != nil {
			return nil, false
		}
		defer reader.Close()
		data, err := io.ReadAll(reader)
		return data, err == nil
	}
	return nil, false
}

func parseSharedStrings(data []byte) []string {
	type run struct {
		Text string `xml:"t"`
	}
	type item struct {
		Text string `xml:"t"`
		Runs []run  `xml:"r"`
	}
	var table struct {
		Items []item `xml:"si"`
	}
	_ = xml.Unmarshal(data, &table)
	result := make([]string, 0, len(table.Items))
	for _, entry := range table.Items {
		value := entry.Text
		for _, part := range entry.Runs {
			value += part.Text
		}
		result = append(result, value)
	}
	return result
}

func parseSheetRows(data []byte, shared []string) ([][]string, error) {
	type cell struct {
		Reference string `xml:"r,attr"`
		Type      string `xml:"t,attr"`
		Value     string `xml:"v"`
		Inline    string `xml:"is>t"`
	}
	type row struct {
		Cells []cell `xml:"c"`
	}
	var sheet struct {
		Rows []row `xml:"sheetData>row"`
	}
	if err := xml.Unmarshal(data, &sheet); err != nil {
		return nil, err
	}
	result := make([][]string, 0, len(sheet.Rows))
	for _, sourceRow := range sheet.Rows {
		maxColumn := 0
		for _, sourceCell := range sourceRow.Cells {
			if column := columnIndex(sourceCell.Reference); column > maxColumn {
				maxColumn = column
			}
		}
		values := make([]string, maxColumn)
		for _, sourceCell := range sourceRow.Cells {
			value := sourceCell.Value
			if sourceCell.Type == "inlineStr" {
				value = sourceCell.Inline
			} else if sourceCell.Type == "s" {
				index, _ := strconv.Atoi(sourceCell.Value)
				if index >= 0 && index < len(shared) {
					value = shared[index]
				}
			}
			column := columnIndex(sourceCell.Reference)
			if column > 0 {
				values[column-1] = value
			}
		}
		result = append(result, values)
	}
	return result, nil
}

func columnName(index int) string {
	var output string
	for index > 0 {
		index--
		output = string(rune('A'+index%26)) + output
		index /= 26
	}
	return output
}

func columnIndex(reference string) int {
	index := 0
	for _, value := range reference {
		if value < 'A' || value > 'Z' {
			break
		}
		index = index*26 + int(value-'A'+1)
	}
	return index
}
