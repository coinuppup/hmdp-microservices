package utils

import (
	"regexp"
)

// 正则表达式
var (
	phoneRegex = regexp.MustCompile(`^1[3-9]\d{9}$`)
)

// IsPhoneInvalid 验证手机号是否无效
func IsPhoneInvalid(phone string) bool {
	return !phoneRegex.MatchString(phone)
}
