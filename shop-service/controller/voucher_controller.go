package controller

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"hmdp-microservices/shop-service/model"
	"hmdp-microservices/shop-service/service"
)

// VoucherController 优惠券控制器
type VoucherController struct {
	voucherService *service.VoucherService
}

// NewVoucherController 创建优惠券控制器
func NewVoucherController(voucherService *service.VoucherService) *VoucherController {
	return &VoucherController{voucherService: voucherService}
}

// ListVouchers 获取优惠券列表
func (c *VoucherController) ListVouchers(ctx *gin.Context) {
	shopIdStr := ctx.Query("shopId")
	shopId, err := strconv.ParseInt(shopIdStr, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误"})
		return
	}

	vouchers, err := c.voucherService.ListVouchers(ctx, shopId)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"code": 200, "message": "OK", "data": vouchers})
}

// CreateVoucher 创建优惠券
func (c *VoucherController) CreateVoucher(ctx *gin.Context) {
	var voucher model.Voucher
	if err := ctx.ShouldBindJSON(&voucher); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误"})
		return
	}

	id, err := c.voucherService.CreateVoucher(ctx, &voucher)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"code": 200, "message": "OK", "data": id})
}

// CreateSeckillVoucher 创建秒杀优惠券
func (c *VoucherController) CreateSeckillVoucher(ctx *gin.Context) {
	type SeckillVoucherRequest struct {
		Voucher model.Voucher `json:"voucher"`
		Stock   int           `json:"stock"`
	}

	var req SeckillVoucherRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误"})
		return
	}

	id, err := c.voucherService.CreateSeckillVoucher(ctx, &req.Voucher, req.Stock)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"code": 200, "message": "OK", "data": id})
}
