package service

import (
	"context"
	"hmdp-microservices/content-service/model"
	"hmdp-microservices/content-service/repository"
	"hmdp-microservices/content-service/utils"
	"strconv"

	"github.com/redis/go-redis/v9"
)

// FollowService 关注服务
type FollowService struct {
	blogRepo *repository.BlogRepository
	rdb      *redis.Client
}

// NewFollowService 创建关注服务
func NewFollowService(blogRepo *repository.BlogRepository, rdb *redis.Client) *FollowService {
	return &FollowService{
		blogRepo: blogRepo,
		rdb:      rdb,
	}
}

// FollowUser 关注用户
func (s *FollowService) FollowUser(ctx context.Context, userId, followUserId int64, isFollow bool) error {
	key := utils.FollowsKey + strconv.FormatInt(userId, 10)

	if isFollow {
		// 关注，新增数据
		follow := &model.Follow{
			UserID:       userId,
			FollowUserID: followUserId,
		}

		err := s.blogRepo.CreateFollow(follow)
		if err == nil {
			// 保存到Redis
			s.rdb.SAdd(ctx, key, strconv.FormatInt(followUserId, 10))
		}

		return err
	} else {
		// 取消关注，删除数据
		err := s.blogRepo.DeleteFollow(userId, followUserId)
		if err == nil {
			// 从Redis中删除
			s.rdb.SRem(ctx, key, strconv.FormatInt(followUserId, 10))
		}

		return err
	}
}

// UnfollowUser 取消关注
func (s *FollowService) UnfollowUser(ctx context.Context, userId, followUserId int64) error {
	return s.FollowUser(ctx, userId, followUserId, false)
}

// ListFollowers 获取粉丝列表
func (s *FollowService) ListFollowers(ctx context.Context, userId int64, current, size int32) ([]map[string]interface{}, int32, error) {
	// 计算偏移量
	offset := (current - 1) * size

	// 查询粉丝列表
	followers, total, err := s.blogRepo.GetFollowers(userId, offset, size)
	if err != nil {
		return nil, 0, err
	}

	// 构建返回结果
	result := make([]map[string]interface{}, len(followers))
	for i, follower := range followers {
		result[i] = map[string]interface{}{
			"id":         follower.UserID,
			"name":       "",
			"icon":       "",
			"followTime": follower.CreateTime,
		}
	}

	return result, total, nil
}

// ListFollowings 获取关注列表
func (s *FollowService) ListFollowings(ctx context.Context, userId int64, current, size int32) ([]map[string]interface{}, int32, error) {
	// 计算偏移量
	offset := (current - 1) * size

	// 查询关注列表
	followings, total, err := s.blogRepo.GetFollowings(userId, offset, size)
	if err != nil {
		return nil, 0, err
	}

	// 构建返回结果
	result := make([]map[string]interface{}, len(followings))
	for i, following := range followings {
		result[i] = map[string]interface{}{
			"id":         following.FollowUserID,
			"name":       "",
			"icon":       "",
			"followTime": following.CreateTime,
		}
	}

	return result, total, nil
}

// ListCommonFollows 获取共同关注
func (s *FollowService) ListCommonFollows(ctx context.Context, userId, targetUserId int64) ([]map[string]interface{}, error) {
	key1 := utils.FollowsKey + strconv.FormatInt(userId, 10)
	key2 := utils.FollowsKey + strconv.FormatInt(targetUserId, 10)

	// 求交集
	intersect, err := s.rdb.SInter(ctx, key1, key2).Result()
	if err != nil {
		return nil, err
	}

	if len(intersect) == 0 {
		return []map[string]interface{}{}, nil
	}

	// 解析用户ID
	var userIDs []int64
	for _, idStr := range intersect {
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err == nil {
			userIDs = append(userIDs, id)
		}
	}

	// TODO: 根据ID查询用户信息
	// 这里暂时返回空列表，后续需要实现
	return []map[string]interface{}{}, nil
}

// CheckFollow 检查是否关注
func (s *FollowService) CheckFollow(ctx context.Context, userId, targetUserId int64) (bool, error) {
	key := utils.FollowsKey + strconv.FormatInt(userId, 10)

	// 从Redis中检查
	exists, err := s.rdb.SIsMember(ctx, key, strconv.FormatInt(targetUserId, 10)).Result()
	if err != nil {
		// Redis查询失败，从数据库查询
		return s.blogRepo.IsFollowed(userId, targetUserId)
	}

	return exists, nil
}
