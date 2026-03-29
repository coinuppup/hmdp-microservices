package service

import (
	"context"
	"fmt"
	"hmdp-microservices/content-service/model"
	"hmdp-microservices/content-service/repository"
	"hmdp-microservices/content-service/utils"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// BlogService 博客服务
type BlogService struct {
	blogRepo *repository.BlogRepository
	rdb      *redis.Client
}

// NewBlogService 创建博客服务
func NewBlogService(blogRepo *repository.BlogRepository, rdb *redis.Client) *BlogService {
	return &BlogService{
		blogRepo: blogRepo,
		rdb:      rdb,
	}
}

// GetBlog 获取博客信息
func (s *BlogService) GetBlog(ctx context.Context, id int64) (map[string]interface{}, error) {
	// 查询博客
	blog, err := s.blogRepo.FindByID(id)
	if err != nil {
		return nil, err
	}

	if blog == nil {
		return nil, fmt.Errorf("博客不存在")
	}

	// 构建返回结果
	result := map[string]interface{}{
		"id":           blog.ID,
		"title":        blog.Title,
		"content":      blog.Content,
		"images":       blog.Images,
		"userId":       blog.UserID,
		"name":         blog.Name,
		"icon":         blog.Icon,
		"liked":        false,
		"likeCount":    0,
		"commentCount": 0,
		"createTime":   blog.CreateTime,
	}

	// 检查是否点赞
	key := utils.BlogLikedKey + strconv.FormatInt(id, 10)
	userID, ok := ctx.Value("userID").(int64)
	if ok {
		_, err := s.rdb.ZScore(ctx, key, strconv.FormatInt(userID, 10)).Result()
		result["liked"] = err == nil
	}

	// 获取点赞数
	likeCount, err := s.rdb.ZCard(ctx, key).Result()
	if err == nil {
		result["likeCount"] = likeCount
	}

	return result, nil
}

// ListHotBlogs 分页查询热门博客
func (s *BlogService) ListHotBlogs(ctx context.Context, current, size int32) ([]map[string]interface{}, int32, error) {
	// 查询热门博客
	blogs, err := s.blogRepo.FindHot(int(current), int(size))
	if err != nil {
		return nil, 0, err
	}

	// 构建返回结果
	result := make([]map[string]interface{}, len(blogs))
	userID, _ := ctx.Value("userID").(int64)

	for i, blog := range blogs {
		// 检查是否点赞
		key := utils.BlogLikedKey + strconv.FormatInt(blog.ID, 10)
		liked := false
		if userID > 0 {
			_, err := s.rdb.ZScore(ctx, key, strconv.FormatInt(userID, 10)).Result()
			liked = err == nil
		}

		// 获取点赞数
		likeCount, _ := s.rdb.ZCard(ctx, key).Result()

		result[i] = map[string]interface{}{
			"id":           blog.ID,
			"title":        blog.Title,
			"content":      blog.Content,
			"images":       blog.Images,
			"userId":       blog.UserID,
			"name":         blog.Name,
			"icon":         blog.Icon,
			"liked":        liked,
			"likeCount":    likeCount,
			"commentCount": 0,
			"createTime":   blog.CreateTime,
		}
	}

	return result, int32(len(blogs)), nil
}

// ListUserBlogs 查询用户博客
func (s *BlogService) ListUserBlogs(ctx context.Context, userId int64, current, size int32) ([]map[string]interface{}, int32, error) {
	// 查询用户博客
	blogs, err := s.blogRepo.FindByUser(userId, int(current), int(size))
	if err != nil {
		return nil, 0, err
	}

	// 构建返回结果
	result := make([]map[string]interface{}, len(blogs))
	currentUserID, _ := ctx.Value("userID").(int64)

	for i, blog := range blogs {
		// 检查是否点赞
		key := utils.BlogLikedKey + strconv.FormatInt(blog.ID, 10)
		liked := false
		if currentUserID > 0 {
			_, err := s.rdb.ZScore(ctx, key, strconv.FormatInt(currentUserID, 10)).Result()
			liked = err == nil
		}

		// 获取点赞数
		likeCount, _ := s.rdb.ZCard(ctx, key).Result()

		result[i] = map[string]interface{}{
			"id":           blog.ID,
			"title":        blog.Title,
			"content":      blog.Content,
			"images":       blog.Images,
			"userId":       blog.UserID,
			"name":         blog.Name,
			"icon":         blog.Icon,
			"liked":        liked,
			"likeCount":    likeCount,
			"commentCount": 0,
			"createTime":   blog.CreateTime,
		}
	}

	return result, int32(len(blogs)), nil
}

// ListFollowBlogs 获取关注feed
func (s *BlogService) ListFollowBlogs(ctx context.Context, userId int64, current, size int32) ([]map[string]interface{}, int32, error) {
	// 从Redis获取关注的博客
	key := utils.FeedKey + strconv.FormatInt(userId, 10)

	// 计算偏移量
	offset := (current - 1) * size

	// 查询收件箱
	results, err := s.rdb.ZRevRange(ctx, key, int64(offset), int64(offset+size-1)).Result()
	if err != nil {
		return nil, 0, err
	}

	if len(results) == 0 {
		return []map[string]interface{}{}, 0, nil
	}

	// 解析博客ID
	var blogIDs []int64
	for _, result := range results {
		blogID, err := strconv.ParseInt(result, 10, 64)
		if err == nil {
			blogIDs = append(blogIDs, blogID)
		}
	}

	// 查询博客详情
	var blogsResult []map[string]interface{}
	for _, id := range blogIDs {
		blog, err := s.blogRepo.FindByID(id)
		if err == nil && blog != nil {
			// 检查是否点赞
			likeKey := utils.BlogLikedKey + strconv.FormatInt(blog.ID, 10)
			liked := false
			_, err := s.rdb.ZScore(ctx, likeKey, strconv.FormatInt(userId, 10)).Result()
			liked = err == nil

			// 获取点赞数
			likeCount, _ := s.rdb.ZCard(ctx, likeKey).Result()

			blogsResult = append(blogsResult, map[string]interface{}{
				"id":           blog.ID,
				"title":        blog.Title,
				"content":      blog.Content,
				"images":       blog.Images,
				"userId":       blog.UserID,
				"name":         blog.Name,
				"icon":         blog.Icon,
				"liked":        liked,
				"likeCount":    likeCount,
				"commentCount": 0,
				"createTime":   blog.CreateTime,
			})
		}
	}

	// 获取总数
	total, _ := s.rdb.ZCard(ctx, key).Result()

	return blogsResult, int32(total), nil
}

// LikeBlog 点赞博客
func (s *BlogService) LikeBlog(ctx context.Context, blogId, userId int64) error {
	key := utils.BlogLikedKey + strconv.FormatInt(blogId, 10)

	// 检查是否已点赞
	_, err := s.rdb.ZScore(ctx, key, strconv.FormatInt(userId, 10)).Result()

	if err == redis.Nil {
		// 未点赞，执行点赞操作
		// 保存到Redis
		s.rdb.ZAdd(ctx, key, redis.Z{
			Score:  float64(time.Now().Unix()),
			Member: strconv.FormatInt(userId, 10),
		})
	} else if err == nil {
		// 已点赞，返回错误
		return fmt.Errorf("已经点过赞了")
	} else {
		return err
	}

	return nil
}

// UnlikeBlog 取消点赞
func (s *BlogService) UnlikeBlog(ctx context.Context, blogId, userId int64) error {
	key := utils.BlogLikedKey + strconv.FormatInt(blogId, 10)

	// 从Redis中删除
	result, err := s.rdb.ZRem(ctx, key, strconv.FormatInt(userId, 10)).Result()
	if err != nil {
		return err
	}

	if result == 0 {
		return fmt.Errorf("未点赞")
	}

	return nil
}

// CreateBlog 发布博客
func (s *BlogService) CreateBlog(ctx context.Context, userId int64, title, content, images string) (int64, error) {
	// 创建博客
	blog := &model.Blog{
		UserID:     userId,
		Title:      title,
		Content:    content,
		Images:     images,
		CreateTime: time.Now(),
	}

	// 保存博客
	err := s.blogRepo.Create(blog)
	if err != nil {
		return 0, err
	}

	// TODO: 推送博客给所有粉丝
	// 这里需要查询用户的所有粉丝，然后将博客ID添加到每个粉丝的feed中

	return blog.ID, nil
}

// ListBlogComments 获取博客评论
func (s *BlogService) ListBlogComments(ctx context.Context, blogId int64, current, size int32) ([]map[string]interface{}, int32, error) {
	// 查询评论
	comments, err := s.blogRepo.FindCommentsByBlog(blogId, int(current), int(size))
	if err != nil {
		return nil, 0, err
	}

	// 构建返回结果
	result := make([]map[string]interface{}, len(comments))
	for i, comment := range comments {
		result[i] = map[string]interface{}{
			"id":         comment.ID,
			"blogId":     comment.BlogID,
			"userId":     comment.UserID,
			"content":    comment.Content,
			"name":       "",
			"icon":       "",
			"createTime": comment.CreateTime,
		}
	}

	return result, int32(len(comments)), nil
}

// CreateComment 发表评论
func (s *BlogService) CreateComment(ctx context.Context, blogId, userId int64, content string) (int64, error) {
	// 创建评论
	comment := &model.BlogComments{
		BlogID:     blogId,
		UserID:     userId,
		Content:    content,
		CreateTime: time.Now(),
	}

	// 保存评论
	err := s.blogRepo.CreateComment(comment)
	if err != nil {
		return 0, err
	}

	return comment.ID, nil
}
