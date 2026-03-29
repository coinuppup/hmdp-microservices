package model

// UserDTO 用户数据传输对象
type UserDTO struct {
	ID       int64  `json:"id"`
	Phone    string `json:"phone"`
	NickName string `json:"nickName"`
	Icon     string `json:"icon"`
}
