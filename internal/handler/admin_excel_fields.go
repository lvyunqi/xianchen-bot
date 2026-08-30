package handler

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
)

func rowsFromCells(spec resourceSpec, rows [][]string) (any, error) {
	if len(rows) < 2 {
		return spec.NewSlice(), nil
	}
	headers := rows[0]
	slicePointer := spec.NewSlice()
	sliceValue := reflect.ValueOf(slicePointer).Elem()
	for _, cells := range rows[1:] {
		row := spec.New()
		if err := assignExcelFields(row, headers, cells); err != nil {
			return nil, err
		}
		sliceValue.Set(reflect.Append(sliceValue, reflect.ValueOf(row).Elem()))
	}
	return slicePointer, nil
}

func assignExcelFields(target any, headers, cells []string) error {
	value := reflect.ValueOf(target)
	if value.Kind() != reflect.Pointer || value.Elem().Kind() != reflect.Struct {
		return errors.New("Excel目标模型不正确")
	}
	value = value.Elem()
	typeInfo := value.Type()
	fields := make(map[string]reflect.Value)
	for index := 0; index < typeInfo.NumField(); index++ {
		fieldInfo := typeInfo.Field(index)
		name := strings.Split(fieldInfo.Tag.Get("json"), ",")[0]
		if name != "" && name != "-" {
			fields[name] = value.Field(index)
		}
	}
	for index, header := range headers {
		if index >= len(cells) {
			continue
		}
		field, ok := fields[strings.TrimSpace(header)]
		if !ok || !field.CanSet() {
			continue
		}
		if err := setStringField(field, strings.TrimSpace(cells[index])); err != nil {
			return fmt.Errorf("字段%s: %w", header, err)
		}
	}
	return nil
}

func setStringField(field reflect.Value, text string) error {
	if field.Kind() == reflect.Pointer {
		if text == "" {
			return nil
		}
		field.Set(reflect.New(field.Type().Elem()))
		return setStringField(field.Elem(), text)
	}
	if field.Type() == reflect.TypeOf(time.Time{}) {
		if text == "" {
			return nil
		}
		parsed, err := time.Parse(time.RFC3339, text)
		if err != nil {
			parsed, err = time.Parse("2006-01-02 15:04:05", text)
		}
		if err != nil {
			return err
		}
		field.Set(reflect.ValueOf(parsed))
		return nil
	}
	switch field.Kind() {
	case reflect.String:
		field.SetString(text)
	case reflect.Bool:
		field.SetBool(text == "1" || strings.EqualFold(text, "true") || text == "是" || text == "真")
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if text == "" {
			return nil
		}
		value, err := strconv.ParseInt(text, 10, 64)
		if err != nil {
			return err
		}
		field.SetInt(value)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if text == "" {
			return nil
		}
		value, err := strconv.ParseUint(text, 10, 64)
		if err != nil {
			return err
		}
		field.SetUint(value)
	case reflect.Float32, reflect.Float64:
		if text == "" {
			return nil
		}
		value, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return err
		}
		field.SetFloat(value)
	}
	return nil
}
