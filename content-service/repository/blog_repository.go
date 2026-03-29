package repository

import (
	"hmdp-microservices/content-service/model"
	"gorm.io/gorm"
)

// BlogRepository 博客仓库
type BlogRepository struct {
	db *gorm.DB
}

// NewBlogRepository 创建博客仓库
func NewBlogRepository(db *gorm.DB) *BlogRepository {
	return &BlogRepository{db: db}
}

// FindByID 根据ID查询博客
func (r *BlogRepository) FindByID(id int64) (*model.Blog, error) {
	var blog model.Blog
	err := r.db.First(&blog, id).Error
	if err != nil {
		return nil, err
	}
	return &blog, nil
}

// FindHot 查询热门博客
func (r *BlogRepository) FindHot(page, pageSize int) ([]*model.Blog, error) {
	var blogs []*model.Blog
	offset := (page - 1) * pageSize
	err := r.db.Order("liked DESC").Offset(offset).Limit(pageSize).Find(&blogs).Error
	if err != nil {
		return nil, err
	}
	return blogs, nil
}

// FindByUser 查询用户博客
func (r *BlogRepository) FindByUser(userID int64, page, pageSize int) ([]*model.Blog, error) {
	var blogs []*model.Blog
	offset := (page - 1) * pageSize
	err := r.db.Where("user_id = ?", userID).Order("create_time DESC").Offset(offset).Limit(pageSize).Find(&blogs).Error
	if err != nil {
		return nil, err
	}
	return blogs, nil
}

// Create 创建博客
func (r *BlogRepository) Create(blog *model.Blog) error {
	return r.db.Create(blog).Error
}

// Update 更新博客
func (r *BlogRepository) Update(blog *model.Blog) error {
	return r.db.Save(blog).Error
}

// UpdateLiked 更新点赞数
func (r *BlogRepository) UpdateLiked(id int64, increment int) error {
	return r.db.Model(&model.Blog{}).Where("id = ?", id).Update("liked", gorm.Expr("liked + ?", increment)).Error
}

// FindCommentsByBlog 查询博客评论
func (r *BlogRepository) FindCommentsByBlog(blogID int64, page, pageSize int) ([]*model.BlogComments, error) {
	var comments []*model.BlogComments
	offset := (page - 1) * pageSize
	err := r.db.Where("blog_id = ?", blogID).Order("create_time DESC").Offset(offset).Limit(pageSize).Find(&comments).Error
	if err != nil {
		return nil, err
	}
	return comments, nil
}

// CreateComment 创建评论
func (r *BlogRepository) CreateComment(comment *model.BlogComments) error {
	return r.db.Create(comment).Error
}

// CreateFollow 创建关注
func (r *BlogRepository) CreateFollow(follow *model.Follow) error {
	return r.db.Create(follow).Error
}

// DeleteFollow 删除关注
func (r *BlogRepository) DeleteFollow(userID, followUserID int64) error {
	return r.db.Where("user_id = ? AND follow_user_id = ?", userID, followUserID).Delete(&model.Follow{}).Error
}

// IsFollowed 检查是否已关注
func (r *BlogRepository) IsFollowed(userID, followUserID int64) (bool, error) {
	var count int64
	err := r.db.Model(&model.Follow{}).Where("user_id = ? AND follow_user_id = ?", userID, followUserID).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetFollowers 获取粉丝列表
func (r *BlogRepository) GetFollowers(userID int64, offset, limit int32) ([]*model.Follow, int32, error) {
	var follows []*model.Follow
	err := r.db.Where("follow_user_id = ?", userID).Offset(int(offset)).Limit(int(limit)).Find(&follows).Error
	if err != nil {
		return nil, 0, err
	}

	var total int64
	r.db.Model(&model.Follow{}).Where("follow_user_id = ?", userID).Count(&total)

	return follows, int32(total), nil
}

// GetFollowings 获取关注列表
func (r *BlogRepository) GetFollowings(userID int64, offset, limit int32) ([]*model.Follow, int32, error) {
	var follows []*model.Follow
	err := r.db.Where("user_id = ?", userID).Offset(int(offset)).Limit(int(limit)).Find(&follows).Error
	if err != nil {
		return nil, 0, err
	}

	var total int64
	r.db.Model(&model.Follow{}).Where("user_id = ?", userID).Count(&total)

	return follows, int32(total), nil
}
