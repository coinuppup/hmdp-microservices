package controller

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"hmdp-microservices/user-service/model"
	"hmdp-microservices/user-service/service"
	"hmdp-microservices/user-service/utils"
)

// UserController 用户控制器
type UserController struct {
	userService *service.UserService
}

// NewUserController 创建用户控制器
func NewUserController(userService *service.UserService) *UserController {
	return &UserController{userService: userService}
}

// SendCode 发送验证码
func (c *UserController) SendCode(ctx *gin.Context) {
	phone := ctx.Query("phone")
	err := c.userService.SendCode(ctx, phone)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, model.Fail(err.Error()))
		return
	}
	ctx.JSON(http.StatusOK, model.Ok(nil))
}

// Login 登录
func (c *UserController) Login(ctx *gin.Context) {
	var loginForm model.LoginFormDTO
	if err := ctx.ShouldBindJSON(&loginForm); err != nil {
		ctx.JSON(http.StatusBadRequest, model.Fail("参数错误"))
		return
	}

	// 获取设备ID
	deviceID := ctx.GetHeader("X-Device-ID")

	result, err := c.userService.Login(ctx, loginForm.Phone, loginForm.Code, deviceID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, model.Fail(err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, model.Ok(result))
}

// ValidateToken 验证Token并自动续期
func (c *UserController) ValidateToken(ctx *gin.Context) {
	// 从Header获取Access Token
	accessToken := ctx.GetHeader("Authorization")
	if accessToken == "" {
		ctx.JSON(http.StatusUnauthorized, model.Fail("缺少Authorization header"))
		return
	}

	// 移除 "Bearer " 前缀
	accessToken = strings.TrimPrefix(accessToken, "Bearer ")
	if accessToken == "" {
		ctx.JSON(http.StatusUnauthorized, model.Fail("无效的token"))
		return
	}

	user, err := c.userService.ValidateToken(ctx, accessToken)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, model.Fail(err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, model.Ok(user))
}

// RefreshToken 使用Refresh Token刷新Token
func (c *UserController) RefreshToken(ctx *gin.Context) {
	var req struct {
		RefreshToken string `json:"refreshToken"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, model.Fail("参数错误"))
		return
	}

	if req.RefreshToken == "" {
		ctx.JSON(http.StatusBadRequest, model.Fail("refreshToken不能为空"))
		return
	}

	// 获取设备ID
	deviceID := ctx.GetHeader("X-Device-ID")

	result, err := c.userService.RefreshToken(ctx, req.RefreshToken, deviceID)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, model.Fail(err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, model.Ok(result))
}

// GetCurrentUser 获取当前用户
func (c *UserController) GetCurrentUser(ctx *gin.Context) {
	// 从Context获取userID
	userID, exists := ctx.Get("userID")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, model.Fail("未登录"))
		return
	}

	// 获取用户信息
	user, err := c.userService.GetUserInfo(ctx, userID.(int64))
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, model.Fail("未登录"))
		return
	}

	ctx.JSON(http.StatusOK, model.Ok(user))
}

// GetUserInfo 获取用户信息
func (c *UserController) GetUserInfo(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, model.Fail("参数错误"))
		return
	}

	user, err := c.userService.GetUserInfo(ctx, id)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, model.Fail(err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, model.Ok(user))
}

// Sign 签到
func (c *UserController) Sign(ctx *gin.Context) {
	user := utils.GetUser(ctx)
	if user == nil {
		ctx.JSON(http.StatusUnauthorized, model.Fail("未登录"))
		return
	}

	err := c.userService.Sign(ctx, user.ID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, model.Fail(err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, model.Ok(nil))
}

// GetSignCount 获取签到次数
func (c *UserController) GetSignCount(ctx *gin.Context) {
	user := utils.GetUser(ctx)
	if user == nil {
		ctx.JSON(http.StatusUnauthorized, model.Fail("未登录"))
		return
	}

	count, err := c.userService.GetSignCount(ctx, user.ID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, model.Fail(err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, model.Ok(count))
}
