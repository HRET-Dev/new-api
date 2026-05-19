package controller

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// ============================================================
// BEPUsdt 请求/响应结构体（兼容 Epusdt 协议）
// ============================================================

type BEPUsdtCreateOrderRequest struct {
	OrderId     string  `json:"order_id"`
	Amount      float64 `json:"amount"`
	Fiat        string  `json:"fiat,omitempty"`
	NotifyUrl   string  `json:"notify_url"`
	RedirectUrl string  `json:"redirect_url"`
	TradeType   string  `json:"trade_type,omitempty"` // usdt.bep20 / usdt.aptos / usdt.arbitrum
	Signature   string  `json:"signature"`
}

type BEPUsdtOrderData struct {
	TradeId        string `json:"trade_id"`
	OrderId        string `json:"order_id"`
	Amount         string `json:"amount"`
	ActualAmount   string `json:"actual_amount"`
	Fiat           string `json:"fiat"`
	Token          string `json:"token"`
	ExpirationTime int64  `json:"expiration_time"`
	PaymentUrl     string `json:"payment_url"`
	Status         int    `json:"status"`
	TradeType      string `json:"trade_type"`
}

type BEPUsdtCreateOrderResponse struct {
	StatusCode int              `json:"status_code"`
	Message    string           `json:"message"`
	Data       BEPUsdtOrderData `json:"data"`
}

// BEPUsdtNotifyRequest 回调通知请求参数（Epusdt 协议）
type BEPUsdtNotifyRequest struct {
	TradeId            string  `form:"trade_id"             json:"trade_id"`
	OrderId            string  `form:"order_id"             json:"order_id"`
	Amount             float64 `form:"amount"               json:"amount"`
	ActualAmount       float64 `form:"actual_amount"        json:"actual_amount"`
	Token              string  `form:"token"                json:"token"`
	TradeType          string  `form:"trade_type"           json:"trade_type"`
	BlockTransactionId string  `form:"block_transaction_id" json:"block_transaction_id"`
	Status             int     `form:"status"               json:"status"` // 1=待付款 2=支付成功 3=支付超时
	Signature          string  `form:"signature"            json:"signature"`
}

func bepusdtFormatAmount(amount float64) string {
	return strconv.FormatFloat(amount, 'f', -1, 64)
}

// ============================================================
// BEPUsdt 签名算法（Epusdt 兼容协议 MD5 签名）
// ============================================================

// bepusdtSign 计算签名：
//  1. 按参数名 ASCII 字典序排列所有非空参数（排除 signature 本身）
//  2. 拼接为 key=value&... 格式
//  3. 直接追加 API Token
//  4. MD5 取 32 位十六进制（小写）
func bepusdtSign(params map[string]string, token string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		if k == "signature" {
			continue
		}
		if params[k] == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", k, params[k]))
	}
	raw := strings.Join(parts, "&") + token

	h := md5.Sum([]byte(raw))
	return fmt.Sprintf("%x", h)
}

// bepusdtVerifyCallback 验证回调签名
func bepusdtVerifyCallback(req *BEPUsdtNotifyRequest, token string) bool {
	params := map[string]string{
		"trade_id":      req.TradeId,
		"order_id":      req.OrderId,
		"amount":        bepusdtFormatAmount(req.Amount),
		"actual_amount": bepusdtFormatAmount(req.ActualAmount),
		"token":         req.Token,
		"trade_type":    req.TradeType,
		"status":        fmt.Sprintf("%d", req.Status),
	}
	if req.BlockTransactionId != "" {
		params["block_transaction_id"] = req.BlockTransactionId
	}
	expected := bepusdtSign(params, token)
	return strings.EqualFold(expected, req.Signature)
}

// ============================================================
// BEPUsdt API 调用
// ============================================================

func getBEPUsdtFiat() string {
	fiatCurrency := strings.ToUpper(strings.TrimSpace(setting.BEPUsdtFiatCurrency))
	switch fiatCurrency {
	case "CNY", "USD", "EUR", "GBP", "JPY":
		return fiatCurrency
	}
	return "CNY"
}

func callBEPUsdtCreateOrder(req *BEPUsdtCreateOrderRequest) (*BEPUsdtCreateOrderResponse, error) {
	apiUrl := strings.TrimRight(setting.BEPUsdtApiUrl, "/") + "/api/v1/order/create-transaction"

	jsonData, err := common.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %v", err)
	}

	httpReq, err := http.NewRequest("POST", apiUrl, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("创建HTTP请求失败: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("HTTP状态异常: status=%d body=%s", resp.StatusCode, string(body))
	}

	var result BEPUsdtCreateOrderResponse
	if err := common.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v, body=%s", err, string(body))
	}
	return &result, nil
}

// ============================================================
// Handler：查询支付金额
// ============================================================

type BEPUsdtAmountRequest struct {
	Amount int64 `json:"amount"`
}

func RequestBEPUsdtAmount(c *gin.Context) {
	var req BEPUsdtAmountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "参数错误"})
		return
	}
	if req.Amount < getMinTopup() {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": fmt.Sprintf("充值数量不能小于 %d", getMinTopup())})
		return
	}
	id := c.GetInt("id")
	group, err := model.GetUserGroup(id, true)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "获取用户分组失败"})
		return
	}
	payMoney := getPayMoney(req.Amount, group)
	if payMoney <= 0.01 {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "充值金额过低"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "success", "data": fmt.Sprintf("%.2f", payMoney)})
}

// ============================================================
// Handler：发起 BEPUsdt 支付
// ============================================================

type BEPUsdtPayRequest struct {
	Amount        int64  `json:"amount"`
	PaymentMethod string `json:"payment_method"`
}

func RequestBEPUsdtPay(c *gin.Context) {
	if !setting.BEPUsdtEnabled {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "BEPUsdt 支付未启用"})
		return
	}

	var req BEPUsdtPayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "参数错误"})
		return
	}
	if req.Amount < getMinTopup() {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": fmt.Sprintf("充值数量不能小于 %d", getMinTopup())})
		return
	}
	tradeType := strings.TrimSpace(req.PaymentMethod)
	if tradeType == "" {
		tradeType = setting.BEPUsdtTradeType
	}
	if !setting.IsBEPUsdtTradeType(tradeType) {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "不支持的 BEPUsdt 交易类型"})
		return
	}
	if !operation_setting.ContainsPayMethod(tradeType) && tradeType != setting.BEPUsdtTradeType {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "BEPUsdt 支付方式未开放"})
		return
	}

	id := c.GetInt("id")
	group, err := model.GetUserGroup(id, true)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "获取用户分组失败"})
		return
	}

	payMoney := getPayMoney(req.Amount, group)
	if payMoney < 0.01 {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "充值金额过低"})
		return
	}

	// 生成唯一订单号
	tradeNo := fmt.Sprintf("BEPUSR%dNO%s%d", id, common.GetRandomString(6), time.Now().Unix())

	// Token 模式下归一化 Amount
	amount := req.Amount
	if operation_setting.GetQuotaDisplayType() == operation_setting.QuotaDisplayTypeTokens {
		amount = int64(float64(req.Amount) / common.QuotaPerUnit)
		if amount < 1 {
			amount = 1
		}
	}

	// 创建本地订单
	topUp := &model.TopUp{
		UserId:          id,
		Amount:          amount,
		Money:           payMoney,
		TradeNo:         tradeNo,
		PaymentMethod:   tradeType,
		PaymentProvider: model.PaymentProviderBEPUsdt,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
	}
	if err := topUp.Insert(); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("BEPUsdt 创建充值订单失败 user_id=%d trade_no=%s error=%q", id, tradeNo, err.Error()))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "创建订单失败"})
		return
	}

	// 构造回调/跳转 URL
	callbackAddress := service.GetCallbackAddress()
	notifyUrl := callbackAddress + "/api/user/bepusdt/notify"
	redirectUrl := paymentReturnPath("/console/log")

	// 构造签名参数
	fiat := getBEPUsdtFiat()
	signParams := map[string]string{
		"order_id":     tradeNo,
		"amount":       bepusdtFormatAmount(payMoney),
		"fiat":         fiat,
		"notify_url":   notifyUrl,
		"redirect_url": redirectUrl,
	}
	if tradeType != "" {
		signParams["trade_type"] = tradeType
	}
	signature := bepusdtSign(signParams, setting.BEPUsdtToken)

	orderReq := &BEPUsdtCreateOrderRequest{
		OrderId:     tradeNo,
		Amount:      payMoney,
		Fiat:        fiat,
		NotifyUrl:   notifyUrl,
		RedirectUrl: redirectUrl,
		TradeType:   tradeType,
		Signature:   signature,
	}

	orderResp, err := callBEPUsdtCreateOrder(orderReq)
	if err != nil {
		_ = model.UpdatePendingTopUpStatus(tradeNo, model.PaymentProviderBEPUsdt, common.TopUpStatusExpired)
		logger.LogError(c.Request.Context(), fmt.Sprintf("BEPUsdt 拉起支付失败 user_id=%d trade_no=%s error=%q", id, tradeNo, err.Error()))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "拉起支付失败"})
		return
	}

	if orderResp.StatusCode != 200 || orderResp.Data.PaymentUrl == "" {
		_ = model.UpdatePendingTopUpStatus(tradeNo, model.PaymentProviderBEPUsdt, common.TopUpStatusExpired)
		logger.LogError(c.Request.Context(), fmt.Sprintf("BEPUsdt API 返回错误 user_id=%d trade_no=%s status_code=%d message=%q", id, tradeNo, orderResp.StatusCode, orderResp.Message))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "拉起支付失败: " + orderResp.Message})
		return
	}

	logger.LogInfo(c.Request.Context(), fmt.Sprintf("BEPUsdt 充值订单创建成功 user_id=%d trade_no=%s amount=%d money=%.2f payment_url=%q", id, tradeNo, req.Amount, payMoney, orderResp.Data.PaymentUrl))
	c.JSON(http.StatusOK, gin.H{"message": "success", "data": gin.H{"payment_url": orderResp.Data.PaymentUrl}})
}

// ============================================================
// Handler：BEPUsdt 支付回调通知
// ============================================================

func BEPUsdtNotify(c *gin.Context) {
	if !isBEPUsdtWebhookEnabled() {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("BEPUsdt webhook 被拒绝 reason=webhook_disabled path=%q client_ip=%s", c.Request.RequestURI, c.ClientIP()))
		_, _ = c.Writer.WriteString("fail")
		return
	}

	var req BEPUsdtNotifyRequest
	// 兼容 POST form 和 JSON
	if c.ContentType() == "application/json" {
		if err := c.ShouldBindJSON(&req); err != nil {
			logger.LogError(c.Request.Context(), fmt.Sprintf("BEPUsdt webhook JSON 解析失败 error=%q", err.Error()))
			_, _ = c.Writer.WriteString("fail")
			return
		}
	} else {
		if err := c.ShouldBind(&req); err != nil {
			logger.LogError(c.Request.Context(), fmt.Sprintf("BEPUsdt webhook form 解析失败 error=%q", err.Error()))
			_, _ = c.Writer.WriteString("fail")
			return
		}
	}

	logger.LogInfo(c.Request.Context(), fmt.Sprintf("BEPUsdt webhook 收到请求 path=%q client_ip=%s order_id=%q status=%d", c.Request.RequestURI, c.ClientIP(), req.OrderId, req.Status))

	// 验证签名
	if !bepusdtVerifyCallback(&req, setting.BEPUsdtToken) {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("BEPUsdt webhook 签名验证失败 order_id=%q amount=%s actual_amount=%s trade_type=%q status=%d client_ip=%s", req.OrderId, bepusdtFormatAmount(req.Amount), bepusdtFormatAmount(req.ActualAmount), req.TradeType, req.Status, c.ClientIP()))
		_, _ = c.Writer.WriteString("fail")
		return
	}

	// 只处理支付成功（status == 2）
	if req.Status != 2 {
		logger.LogInfo(c.Request.Context(), fmt.Sprintf("BEPUsdt webhook 忽略非成功状态 order_id=%q status=%d", req.OrderId, req.Status))
		if req.Status == 3 {
			_ = model.UpdatePendingTopUpStatus(req.OrderId, model.PaymentProviderBEPUsdt, common.TopUpStatusExpired)
		}
		_, _ = c.Writer.WriteString("ok")
		return
	}

	LockOrder(req.OrderId)
	defer UnlockOrder(req.OrderId)

	topUp := model.GetTopUpByTradeNo(req.OrderId)
	if topUp == nil {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("BEPUsdt webhook 订单不存在 order_id=%q client_ip=%s", req.OrderId, c.ClientIP()))
		_, _ = c.Writer.WriteString("fail")
		return
	}
	if topUp.PaymentProvider != model.PaymentProviderBEPUsdt {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("BEPUsdt webhook 支付渠道不匹配 order_id=%q provider=%s client_ip=%s", req.OrderId, topUp.PaymentProvider, c.ClientIP()))
		_, _ = c.Writer.WriteString("fail")
		return
	}
	if topUp.Status != common.TopUpStatusPending {
		logger.LogInfo(c.Request.Context(), fmt.Sprintf("BEPUsdt webhook 订单状态非 pending，忽略 order_id=%q status=%s", req.OrderId, topUp.Status))
		_, _ = c.Writer.WriteString("ok")
		return
	}

	topUp.Status = common.TopUpStatusSuccess
	if err := topUp.Update(); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("BEPUsdt 更新充值订单失败 order_id=%q user_id=%d error=%q", req.OrderId, topUp.UserId, err.Error()))
		_, _ = c.Writer.WriteString("fail")
		return
	}

	dAmount := decimal.NewFromInt(int64(topUp.Amount))
	dQuotaPerUnit := decimal.NewFromFloat(common.QuotaPerUnit)
	quotaToAdd := int(dAmount.Mul(dQuotaPerUnit).IntPart())

	if err := model.IncreaseUserQuota(topUp.UserId, quotaToAdd, true); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("BEPUsdt 增加用户配额失败 order_id=%q user_id=%d quota_to_add=%d error=%q", req.OrderId, topUp.UserId, quotaToAdd, err.Error()))
		_, _ = c.Writer.WriteString("fail")
		return
	}

	logger.LogInfo(c.Request.Context(), fmt.Sprintf("BEPUsdt 充值成功 order_id=%q user_id=%d quota_to_add=%d money=%.2f", req.OrderId, topUp.UserId, quotaToAdd, topUp.Money))
	model.RecordTopupLog(topUp.UserId, fmt.Sprintf("使用 BEPUsdt 充值成功，充值金额: %v，支付金额：%.4f USDT", logger.LogQuota(quotaToAdd), topUp.Money), c.ClientIP(), topUp.PaymentMethod, model.PaymentProviderBEPUsdt)

	_, _ = c.Writer.WriteString("ok")
}
