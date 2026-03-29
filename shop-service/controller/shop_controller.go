package controller

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"hmdp-microservices/shop-service/model"
	"hmdp-microservices/shop-service/service"
)

// ShopController 商铺控制器
type ShopController struct {
	shopService *service.ShopService
}

// NewShopController 创建商铺控制器
func NewShopController(shopService *service.ShopService) *ShopController {
	return &ShopController{shopService: shopService}
}

// GetShop 获取商铺
func (c *ShopController) GetShop(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误"})
		return
	}

	shop, err := c.shopService.GetShop(ctx, id)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"code": 200, "message": "OK", "data": shop})
}

// ListShops 分页查询商铺
func (c *ShopController) ListShops(ctx *gin.Context) {
	typeIdStr := ctx.Query("typeId")
	currentStr := ctx.Query("current")

	typeId, err := strconv.ParseInt(typeIdStr, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误"})
		return
	}

	current, err := strconv.ParseInt(currentStr, 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误"})
		return
	}

	shops, err := c.shopService.ListShops(ctx, typeId, int32(current), 5)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"code": 200, "message": "OK", "data": shops})
}

// CreateShop 创建商铺
func (c *ShopController) CreateShop(ctx *gin.Context) {
	var shop model.Shop
	if err := ctx.ShouldBindJSON(&shop); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误"})
		return
	}

	id, err := c.shopService.CreateShop(ctx, &shop)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"code": 200, "message": "OK", "data": id})
}

// UpdateShop 更新商铺
func (c *ShopController) UpdateShop(ctx *gin.Context) {
	var shop model.Shop
	if err := ctx.ShouldBindJSON(&shop); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误"})
		return
	}

	err := c.shopService.UpdateShop(ctx, &shop)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"code": 200, "message": "OK"})
}

// DeleteShop 删除商铺
func (c *ShopController) DeleteShop(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误"})
		return
	}

	err = c.shopService.DeleteShop(ctx, id)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"code": 200, "message": "OK"})
}

// ListShopTypes 获取商铺类型
func (c *ShopController) ListShopTypes(ctx *gin.Context) {
	shopTypes, err := c.shopService.ListShopTypes(ctx)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"code": 200, "message": "OK", "data": shopTypes})
}
