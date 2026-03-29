package controller

import (
	"hmdp-microservices/content-service/service"
	"hmdp-microservices/content-service/utils"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// FollowController 关注控制器
type FollowController struct {
	followService *service.FollowService
}

// NewFollowController 创建关注控制器
func NewFollowController(followService *service.FollowService) *FollowController {
	return &FollowController{
		followService: followService,
	}
}

// Register 注册路由
func (c *FollowController) Register(router *gin.RouterGroup) {
	follow := router.Group("/follow")
	{
		follow.POST("/user", c.FollowUser)
		follow.GET("/followers", c.ListFollowers)
		follow.GET("/followings", c.ListFollowings)
		follow.GET("/common", c.ListCommonFollows)
		follow.GET("/check", c.CheckFollow)
	}
}

// FollowUser 关注用户
func (c *FollowController) FollowUser(ctx *gin.Context) {
	var req struct {
		FollowUserID int64 `json:"followUserId" binding:"required"`
		IsFollow     bool  `json:"isFollow"`
	}

	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数"})
		return
	}

	userID := utils.GetUserID(ctx)
	if userID == 0 {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	err := c.followService.FollowUser(ctx.Request.Context(), userID, req.FollowUserID, req.IsFollow)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	message := "关注成功"
	if !req.IsFollow {
		message = "取消关注成功"
	}

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": message,
	})
}

// ListFollowers 获取粉丝列表
func (c *FollowController) ListFollowers(ctx *gin.Context) {
	userIdStr := ctx.Query("userId")
	currentStr := ctx.DefaultQuery("current", "1")
	sizeStr := ctx.DefaultQuery("size", "10")

	userId, err := strconv.ParseInt(userIdStr, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的用户ID"})
		return
	}

	current, err := strconv.ParseInt(currentStr, 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的current参数"})
		return
	}

	size, err := strconv.ParseInt(sizeStr, 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的size参数"})
		return
	}

	followers, total, err := c.followService.ListFollowers(ctx.Request.Context(), userId, int32(current), int32(size))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    followers,
		"total":   total,
	})
}

// ListFollowings 获取关注列表
func (c *FollowController) ListFollowings(ctx *gin.Context) {
	userIdStr := ctx.Query("userId")
	currentStr := ctx.DefaultQuery("current", "1")
	sizeStr := ctx.DefaultQuery("size", "10")

	userId, err := strconv.ParseInt(userIdStr, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的用户ID"})
		return
	}

	current, err := strconv.ParseInt(currentStr, 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的current参数"})
		return
	}

	size, err := strconv.ParseInt(sizeStr, 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的size参数"})
		return
	}

	followings, total, err := c.followService.ListFollowings(ctx.Request.Context(), userId, int32(current), int32(size))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    followings,
		"total":   total,
	})
}

// ListCommonFollows 获取共同关注
func (c *FollowController) ListCommonFollows(ctx *gin.Context) {
	targetUserIdStr := ctx.Query("targetUserId")
	targetUserId, err := strconv.ParseInt(targetUserIdStr, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的目标用户ID"})
		return
	}

	userID := utils.GetUserID(ctx)
	if userID == 0 {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	commons, err := c.followService.ListCommonFollows(ctx.Request.Context(), userID, targetUserId)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    commons,
	})
}

// CheckFollow 检查是否关注
func (c *FollowController) CheckFollow(ctx *gin.Context) {
	targetUserIdStr := ctx.Query("targetUserId")
	targetUserId, err := strconv.ParseInt(targetUserIdStr, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的目标用户ID"})
		return
	}

	userID := utils.GetUserID(ctx)
	if userID == 0 {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	isFollow, err := c.followService.CheckFollow(ctx.Request.Context(), userID, targetUserId)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    isFollow,
	})
}
