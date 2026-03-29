package controller

import (
	"hmdp-microservices/content-service/service"
	"hmdp-microservices/content-service/utils"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// BlogController 博客控制器
type BlogController struct {
	blogService *service.BlogService
}

// NewBlogController 创建博客控制器
func NewBlogController(blogService *service.BlogService) *BlogController {
	return &BlogController{
		blogService: blogService,
	}
}

// Register 注册路由
func (c *BlogController) Register(router *gin.RouterGroup) {
	blog := router.Group("/blog")
	{
		blog.GET("/hot", c.ListHotBlogs)
		blog.GET("/user", c.ListUserBlogs)
		blog.GET("/follow", c.ListFollowBlogs)
		blog.GET("/:id", c.GetBlog)
		blog.POST("/like", c.LikeBlog)
		blog.POST("/unlike", c.UnlikeBlog)
		blog.POST("", c.CreateBlog)
		blog.GET("/:id/comments", c.ListBlogComments)
		blog.POST("/:id/comments", c.CreateComment)
	}
}

// GetBlog 获取博客信息
func (c *BlogController) GetBlog(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的博客ID"})
		return
	}

	userID := utils.GetUserID(ctx)
	ctx.Request = ctx.Request.WithContext(utils.SetUserID(ctx.Request.Context(), userID))

	blog, err := c.blogService.GetBlog(ctx.Request.Context(), id)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    blog,
	})
}

// ListHotBlogs 分页查询热门博客
func (c *BlogController) ListHotBlogs(ctx *gin.Context) {
	currentStr := ctx.DefaultQuery("current", "1")
	sizeStr := ctx.DefaultQuery("size", "10")

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

	userID := utils.GetUserID(ctx)
	ctx.Request = ctx.Request.WithContext(utils.SetUserID(ctx.Request.Context(), userID))

	blogs, total, err := c.blogService.ListHotBlogs(ctx.Request.Context(), int32(current), int32(size))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    blogs,
		"total":   total,
	})
}

// ListUserBlogs 查询用户博客
func (c *BlogController) ListUserBlogs(ctx *gin.Context) {
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

	userID := utils.GetUserID(ctx)
	ctx.Request = ctx.Request.WithContext(utils.SetUserID(ctx.Request.Context(), userID))

	blogs, total, err := c.blogService.ListUserBlogs(ctx.Request.Context(), userId, int32(current), int32(size))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    blogs,
		"total":   total,
	})
}

// ListFollowBlogs 获取关注feed
func (c *BlogController) ListFollowBlogs(ctx *gin.Context) {
	currentStr := ctx.DefaultQuery("current", "1")
	sizeStr := ctx.DefaultQuery("size", "10")

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

	userID := utils.GetUserID(ctx)
	if userID == 0 {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	ctx.Request = ctx.Request.WithContext(utils.SetUserID(ctx.Request.Context(), userID))

	blogs, total, err := c.blogService.ListFollowBlogs(ctx.Request.Context(), userID, int32(current), int32(size))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    blogs,
		"total":   total,
	})
}

// LikeBlog 点赞博客
func (c *BlogController) LikeBlog(ctx *gin.Context) {
	blogIdStr := ctx.Query("blogId")
	blogId, err := strconv.ParseInt(blogIdStr, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的博客ID"})
		return
	}

	userID := utils.GetUserID(ctx)
	if userID == 0 {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	ctx.Request = ctx.Request.WithContext(utils.SetUserID(ctx.Request.Context(), userID))

	err = c.blogService.LikeBlog(ctx.Request.Context(), blogId, userID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "点赞成功",
	})
}

// UnlikeBlog 取消点赞
func (c *BlogController) UnlikeBlog(ctx *gin.Context) {
	blogIdStr := ctx.Query("blogId")
	blogId, err := strconv.ParseInt(blogIdStr, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的博客ID"})
		return
	}

	userID := utils.GetUserID(ctx)
	if userID == 0 {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	ctx.Request = ctx.Request.WithContext(utils.SetUserID(ctx.Request.Context(), userID))

	err = c.blogService.UnlikeBlog(ctx.Request.Context(), blogId, userID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "取消点赞成功",
	})
}

// CreateBlog 发布博客
func (c *BlogController) CreateBlog(ctx *gin.Context) {
	var req struct {
		Title   string `json:"title" binding:"required"`
		Content string `json:"content" binding:"required"`
		Images  string `json:"images"`
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

	ctx.Request = ctx.Request.WithContext(utils.SetUserID(ctx.Request.Context(), userID))

	blogId, err := c.blogService.CreateBlog(ctx.Request.Context(), userID, req.Title, req.Content, req.Images)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    blogId,
		"message": "发布成功",
	})
}

// ListBlogComments 获取博客评论
func (c *BlogController) ListBlogComments(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的博客ID"})
		return
	}

	currentStr := ctx.DefaultQuery("current", "1")
	sizeStr := ctx.DefaultQuery("size", "10")

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

	comments, total, err := c.blogService.ListBlogComments(ctx.Request.Context(), id, int32(current), int32(size))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    comments,
		"total":   total,
	})
}

// CreateComment 发表评论
func (c *BlogController) CreateComment(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的博客ID"})
		return
	}

	var req struct {
		Content string `json:"content" binding:"required"`
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

	ctx.Request = ctx.Request.WithContext(utils.SetUserID(ctx.Request.Context(), userID))

	commentId, err := c.blogService.CreateComment(ctx.Request.Context(), id, userID, req.Content)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    commentId,
		"message": "评论成功",
	})
}
