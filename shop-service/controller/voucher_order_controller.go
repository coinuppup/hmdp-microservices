package controller

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"hmdp-microservices/shop-service/service"
)

// VoucherOrderController 订单控制器
type VoucherOrderController struct {
	voucherOrderService *service.VoucherOrderService
}

// NewVoucherOrderController 创建订单控制器
func NewVoucherOrderController(voucherOrderService *service.VoucherOrderService) *VoucherOrderController {
	return &VoucherOrderController{voucherOrderService: voucherOrderService}
}

// SeckillVoucher 秒杀优惠券下单
func (c *VoucherOrderController) SeckillVoucher(ctx *gin.Context) {
	idStr := ctx.Param("id")
	voucherID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误"})
		return
	}

	// 从上下文获取用户ID（实际项目中应该从认证中间件获取）
	// TODO 从认证中间件获取用户ID
	userID := int64(1) // 模拟用户ID

	orderID, err := c.voucherOrderService.SeckillVoucher(ctx, voucherID, userID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"code": 200, "message": "OK", "data": orderID})
}

// ListOrders 获取订单列表
func (c *VoucherOrderController) ListOrders(ctx *gin.Context) {
	currentStr := ctx.Query("current")
	current, err := strconv.ParseInt(currentStr, 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误"})
		return
	}

	// 从上下文获取用户ID（实际项目中应该从认证中间件获取）
	userID := int64(1) // 模拟用户ID

	orders, err := c.voucherOrderService.ListOrders(ctx, userID, int32(current), 5)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"code": 200, "message": "OK", "data": orders})
}
