package model

// Result 统一响应结构
type Result struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// NewResult 创建响应
func NewResult(code int, message string, data interface{}) *Result {
	return &Result{
		Code:    code,
		Message: message,
		Data:    data,
	}
}

// Ok 成功响应
func Ok(data interface{}) *Result {
	return NewResult(200, "OK", data)
}

// Fail 失败响应
func Fail(message string) *Result {
	return NewResult(400, message, nil)
}
