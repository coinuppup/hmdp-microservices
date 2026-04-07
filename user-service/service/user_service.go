package service

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"hmdp-microservices/user-service/model"
	"hmdp-microservices/user-service/repository"
	"hmdp-microservices/user-service/utils"
)

// UserService 用户服务
type UserService struct {
	db           *gorm.DB
	rdb          *redis.Client
	repo         *repository.UserRepository
	tokenService *utils.TokenService
}

// NewUserService 创建用户服务
func NewUserService(db *gorm.DB, rdb *redis.Client, secret string) *UserService {
	return &UserService{
		db:           db,
		rdb:          rdb,
		repo:         repository.NewUserRepository(db),
		tokenService: utils.NewTokenService(rdb, secret),
	}
}

// SendCode 发送验证码
func (s *UserService) SendCode(ctx context.Context, phone string) error {
	// 验证手机号，如果不符合，会返回错误信息
	if utils.IsPhoneInvalid(phone) {
		return fmt.Errorf("手机号格式错误")
	}

	// 生成验证码
	rand.Seed(time.Now().UnixNano())
	code := fmt.Sprintf("%06d", rand.Intn(1000000))

	// 保存验证码到Redis
	key := utils.LoginCodeKey + phone
	err := s.rdb.Set(ctx, key, code, time.Duration(utils.LoginCodeTTL)*time.Minute).Err()
	if err != nil {
		return err
	}

	// 模拟发送验证码
	fmt.Printf("发送验证码成功，验证码：%s\n", code)
	return nil
}

// Login 用户登录
// deviceID: 设备ID，用于多设备管理，可为空
func (s *UserService) Login(ctx context.Context, phone, code, deviceID string) (interface{}, error) {
	// 验证手机号
	if utils.IsPhoneInvalid(phone) {
		return "", fmt.Errorf("手机号格式错误")
	}

	// 从Redis获取验证码，查询验证码是否正确
	key := utils.LoginCodeKey + phone
	cacheCode, err := s.rdb.Get(ctx, key).Result()
	if err != nil {
		return "", fmt.Errorf("验证码错误")
	}
	if cacheCode != code {
		return "", fmt.Errorf("验证码错误")
	}

	// 查询用户
	user, err := s.repo.FindByPhone(phone)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// 创建新用户
			user = &model.User{
				Phone:    phone,
				NickName: "用户" + phone[len(phone)-4:],
			}
			err = s.repo.Create(user)
			if err != nil {
				return "", err
			}
		} else {
			return "", err
		}
	}

	// 设置默认设备ID
	if deviceID == "" {
		deviceID = "default"
	}

	// 生成双Token
	tokenPair, err := s.tokenService.GenerateTokenPair(ctx, user.ID, deviceID)
	if err != nil {
		return nil, err
	}

	// 保存用户信息到Redis (用于快速获取用户信息)
	userDTO := &model.UserDTO{
		ID:       user.ID,
		Phone:    user.Phone,
		NickName: user.NickName,
		Icon:     user.Icon,
	}

	userKey := utils.LoginUserKey + fmt.Sprintf("%d", user.ID)
	s.rdb.HSet(ctx, userKey, map[string]interface{}{
		"id":       userDTO.ID,
		"phone":    userDTO.Phone,
		"nickName": userDTO.NickName,
		"icon":     userDTO.Icon,
	})
	s.rdb.Expire(ctx, userKey, time.Duration(utils.LoginUserTTL)*time.Minute)

	// 返回双Token + 用户信息
	return map[string]interface{}{
		"accessToken":  tokenPair.AccessToken,
		"refreshToken": tokenPair.RefreshToken,
		"expiresIn":    tokenPair.ExpiresIn,
		"tokenType":    tokenPair.TokenType,
		"user":         userDTO,
	}, nil
}

// GetUserInfo 获取用户信息
func (s *UserService) GetUserInfo(ctx context.Context, userId int64) (*model.UserDTO, error) {
	user, err := s.repo.FindByID(userId)
	if err != nil {
		return nil, err
	}

	userDTO := &model.UserDTO{
		ID:       user.ID,
		Phone:    user.Phone,
		NickName: user.NickName,
		Icon:     user.Icon,
	}

	return userDTO, nil
}

// Sign 用户签到
func (s *UserService) Sign(ctx context.Context, userId int64) error {
	// 获取日期
	now := time.Now()
	yyyyMM := now.Format("2006:01")
	key := utils.UserSignKey + yyyyMM + ":" + strconv.FormatInt(userId, 10)

	// 获取今天是本月的第几天
	dayOfMonth := now.Day()

	// 写入Redis
	err := s.rdb.SetBit(ctx, key, int64(dayOfMonth-1), 1).Err()
	if err != nil {
		return err
	}

	return nil
}

// GetSignCount 获取签到次数
func (s *UserService) GetSignCount(ctx context.Context, userId int64) (int32, error) {
	// 获取日期
	now := time.Now()
	yyyyMM := now.Format("2006:01")
	key := utils.UserSignKey + yyyyMM + ":" + strconv.FormatInt(userId, 10)

	// 获取今天是本月的第几天
	dayOfMonth := now.Day()

	// 获取截至本月今天的所有签到记录
	result, err := s.rdb.BitField(ctx, key, "GET", "u"+strconv.Itoa(dayOfMonth), "0").Result()
	if err != nil {
		return 0, err
	}

	if len(result) == 0 {
		return 0, nil
	}

	num := result[0]
	if num == 0 {
		return 0, nil
	}

	// 计算连续签到天数
	count := int32(0)
	binaryStr := strconv.FormatInt(num, 2)
	for i := len(binaryStr) - 1; i >= 0; i-- {
		if binaryStr[i] == '1' {
			count++
		} else {
			break
		}
	}

	return count, nil
}

// ValidateToken 验证Token并返回用户信息（自动续期）
func (s *UserService) ValidateToken(ctx context.Context, accessToken string) (*model.UserDTO, error) {
	// 验证Access Token
	tokenInfo, err := s.tokenService.ValidateAccessToken(ctx, accessToken)
	if err != nil {
		return nil, err
	}

	// 获取用户信息
	user, err := s.repo.FindByID(tokenInfo.UserID)
	if err != nil {
		return nil, err
	}

	return &model.UserDTO{
		ID:       user.ID,
		Phone:    user.Phone,
		NickName: user.NickName,
		Icon:     user.Icon,
	}, nil
}

// RefreshToken 使用Refresh Token刷新Access Token
// deviceID: 设备ID，用于多设备管理，可为空
func (s *UserService) RefreshToken(ctx context.Context, refreshToken, deviceID string) (map[string]interface{}, error) {
	// 设置默认设备ID
	if deviceID == "" {
		deviceID = "default"
	}

	// 使用Refresh Token换发新的双Token
	tokenPair, err := s.tokenService.RefreshToken(ctx, refreshToken, deviceID)
	if err != nil {
		return nil, err
	}

	// 返回新的Token对
	return map[string]interface{}{
		"accessToken":  tokenPair.AccessToken,
		"refreshToken": tokenPair.RefreshToken,
		"expiresIn":    tokenPair.ExpiresIn,
		"tokenType":    tokenPair.TokenType,
	}, nil
}
