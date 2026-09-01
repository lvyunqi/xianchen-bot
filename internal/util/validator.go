package util

import (
	"errors"
	"regexp"
	"strings"
)

var daoNamePattern = regexp.MustCompile(`^[\p{Han}A-Za-z0-9_]{2,16}$`)

func ValidateDaoName(value string) error {
	if !daoNamePattern.MatchString(strings.TrimSpace(value)) {
		return errors.New("道号必须为2到16个汉字、字母、数字或下划线")
	}
	return nil
}
func RequireConfirm(got, want string) error {
	if got != want {
		return errors.New("危险操作确认字段不匹配")
	}
	return nil
}
