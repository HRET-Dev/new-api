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
// Epusdt 请求/响应结构体
// ============================================================

type EpusdtCreateOrderRequest struct {
	Pid         string  `json:"pid,omitempty"`
	OrderId     string  `json:"order_id"`
	Currency    string  `json:"currency,omitempty"`
	Token       string  `json:"token,omitempty"`
	Network     string  `json:"network,omitempty"`
	Amount      float64 `json:"amount"`
	NotifyUrl   string  `json:"notify_url"`
	RedirectUrl string  `json:"redirect_url,omitempty"`
	Signature   string  `json:"signature"`
}

type EpusdtOrderData struct {
	TradeId        string `json:"trade_id"`
	OrderId        string `json:"order_id"`
	Amount         any    `json:"amount"`
	Currency       string `json:"currency"`
	ActualAmount   any    `json:"actual_amount"`
	ReceiveAddress string `json:"receive_address"`
	Token          string `json:"token"`
	ExpirationTime int64  `json:"expiration_time"`
	PaymentUrl     string `json:"payment_url"`
}

type EpusdtCreateOrderResponse struct {
	StatusCode int             `json:"status_code"`
	Message    string          `json:"message"`
	Data       EpusdtOrderData `json:"data"`
}

// EpusdtNotifyRequest 回调通知请求参数（Epusdt 协议）
type EpusdtNotifyRequest struct {
	Pid                string  `form:"pid"                  json:"pid"`
	TradeId            string  `form:"trade_id"             json:"trade_id"`
	OrderId            string  `form:"order_id"             json:"order_id"`
	Amount             float64 `form:"amount"               json:"amount"`
	ActualAmount       float64 `form:"actual_amount"        json:"actual_amount"`
	ReceiveAddress     string  `form:"receive_address"      json:"receive_address"`
	Token              string  `form:"token"                json:"token"`
	BlockTransactionId string  `form:"block_transaction_id" json:"block_transaction_id"`
	Status             int     `form:"status"               json:"status"` // 1=待付款 2=支付成功 3=已过期/支付超时
	Signature          string  `form:"signature"            json:"signature"`
}

func epusdtFormatAmount(amount float64) string {
	return strconv.FormatFloat(amount, 'f', -1, 64)
}

// ============================================================
// Epusdt 签名算法（MD5 签名）
// ============================================================

// epusdtSign 计算签名：
//  1. 按参数名 ASCII 字典序排列所有非空参数（排除 signature 本身）
//  2. 拼接为 key=value&... 格式
//  3. 直接追加 secret_key
//  4. MD5 取 32 位十六进制（小写）
func epusdtSign(params map[string]string, token string) string {
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

func epusdtVerifyCallback(req *EpusdtNotifyRequest, token string) bool {
	params := map[string]string{
		"pid":             req.Pid,
		"trade_id":        req.TradeId,
		"order_id":        req.OrderId,
		"amount":          epusdtFormatAmount(req.Amount),
		"actual_amount":   epusdtFormatAmount(req.ActualAmount),
		"receive_address": req.ReceiveAddress,
		"token":           req.Token,
		"status":          fmt.Sprintf("%d", req.Status),
	}
	if req.BlockTransactionId != "" {
		params["block_transaction_id"] = req.BlockTransactionId
	}
	expected := epusdtSign(params, token)
	return strings.EqualFold(expected, req.Signature)
}

// ============================================================
// Epusdt API 调用
// ============================================================

func callEpusdtCreateOrder(req *EpusdtCreateOrderRequest) (*EpusdtCreateOrderResponse, error) {
	apiUrl := strings.TrimRight(setting.EpusdtApiUrl, "/") + "/payments/gmpay/v1/order/create-transaction"

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

	var result EpusdtCreateOrderResponse
	if err := common.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v, body=%s", err, string(body))
	}
	return &result, nil
}

// ============================================================
// Handler：查询支付金额
// ============================================================

type EpusdtAmountRequest struct {
	Amount int64 `json:"amount"`
}

func RequestEpusdtAmount(c *gin.Context) {
	var req EpusdtAmountRequest
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
// Handler：发起 Epusdt 支付
// ============================================================

type EpusdtPayRequest struct {
	Amount        int64  `json:"amount"`
	PaymentMethod string `json:"payment_method"`
}

func RequestEpusdtPay(c *gin.Context) {
	if !setting.EpusdtEnabled {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "Epusdt 支付未启用"})
		return
	}

	var req EpusdtPayRequest
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
		tradeTypes := setting.GetEpusdtTradeTypes()
		if len(tradeTypes) > 0 {
			tradeType = tradeTypes[0]
		}
	}
	if !setting.IsEpusdtTradeType(tradeType) {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "不支持的 Epusdt 交易类型"})
		return
	}
	route := setting.ResolveEpusdtPaymentRoute(tradeType)
	payToken := route.Token
	network := route.Network
	if payToken == "" || network == "" {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "Epusdt 支付网络配置错误"})
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
		PaymentProvider: model.PaymentProviderEpusdt,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
	}
	if err := topUp.Insert(); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Epusdt 创建充值订单失败 user_id=%d trade_no=%s error=%q", id, tradeNo, err.Error()))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "创建订单失败"})
		return
	}

	// 构造回调/跳转 URL
	callbackAddress := service.GetCallbackAddress()
	notifyUrl := callbackAddress + "/api/user/epusdt/notify"
	redirectUrl := paymentReturnPath("/console/log")

	currency := setting.GetEpusdtCurrency()
	signParams := map[string]string{
		"pid":          setting.EpusdtPid,
		"order_id":     tradeNo,
		"currency":     currency,
		"token":        payToken,
		"network":      network,
		"amount":       epusdtFormatAmount(payMoney),
		"notify_url":   notifyUrl,
		"redirect_url": redirectUrl,
	}
	orderReq := &EpusdtCreateOrderRequest{
		Pid:         setting.EpusdtPid,
		OrderId:     tradeNo,
		Currency:    currency,
		Token:       payToken,
		Network:     network,
		Amount:      payMoney,
		NotifyUrl:   notifyUrl,
		RedirectUrl: redirectUrl,
		Signature:   epusdtSign(signParams, setting.GetEpusdtSecretKey()),
	}

	orderResp, err := callEpusdtCreateOrder(orderReq)
	if err != nil {
		_ = model.UpdatePendingTopUpStatus(tradeNo, model.PaymentProviderEpusdt, common.TopUpStatusExpired)
		logger.LogError(c.Request.Context(), fmt.Sprintf("Epusdt 拉起支付失败 user_id=%d trade_no=%s error=%q", id, tradeNo, err.Error()))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "拉起支付失败"})
		return
	}

	if orderResp.StatusCode != 200 || orderResp.Data.PaymentUrl == "" {
		_ = model.UpdatePendingTopUpStatus(tradeNo, model.PaymentProviderEpusdt, common.TopUpStatusExpired)
		logger.LogError(c.Request.Context(), fmt.Sprintf("Epusdt API 返回错误 user_id=%d trade_no=%s status_code=%d message=%q", id, tradeNo, orderResp.StatusCode, orderResp.Message))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "拉起支付失败: " + orderResp.Message})
		return
	}

	logger.LogInfo(c.Request.Context(), fmt.Sprintf("Epusdt 充值订单创建成功 user_id=%d trade_no=%s amount=%d money=%.2f trade_type=%s token=%s network=%s payment_url=%q", id, tradeNo, req.Amount, payMoney, tradeType, payToken, network, orderResp.Data.PaymentUrl))
	c.JSON(http.StatusOK, gin.H{"message": "success", "data": gin.H{"payment_url": orderResp.Data.PaymentUrl}})
}

// ============================================================
// Handler：Epusdt 支付回调通知
// ============================================================

func EpusdtNotify(c *gin.Context) {
	if !isEpusdtWebhookEnabled() {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("Epusdt webhook 被拒绝 reason=webhook_disabled path=%q client_ip=%s", c.Request.RequestURI, c.ClientIP()))
		_, _ = c.Writer.WriteString("fail")
		return
	}

	var req EpusdtNotifyRequest
	// 兼容 POST form 和 JSON
	if c.ContentType() == "application/json" {
		if err := c.ShouldBindJSON(&req); err != nil {
			logger.LogError(c.Request.Context(), fmt.Sprintf("Epusdt webhook JSON 解析失败 error=%q", err.Error()))
			_, _ = c.Writer.WriteString("fail")
			return
		}
	} else {
		if err := c.ShouldBind(&req); err != nil {
			logger.LogError(c.Request.Context(), fmt.Sprintf("Epusdt webhook form 解析失败 error=%q", err.Error()))
			_, _ = c.Writer.WriteString("fail")
			return
		}
	}

	logger.LogInfo(c.Request.Context(), fmt.Sprintf("Epusdt webhook 收到请求 path=%q client_ip=%s order_id=%q status=%d", c.Request.RequestURI, c.ClientIP(), req.OrderId, req.Status))

	// 验证签名
	if req.Pid != strings.TrimSpace(setting.EpusdtPid) {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("Epusdt webhook PID 不匹配 order_id=%q pid=%q client_ip=%s", req.OrderId, req.Pid, c.ClientIP()))
		_, _ = c.Writer.WriteString("fail")
		return
	}
	if !epusdtVerifyCallback(&req, setting.GetEpusdtSecretKey()) {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("Epusdt webhook 签名验证失败 order_id=%q amount=%s actual_amount=%s token=%q status=%d client_ip=%s", req.OrderId, epusdtFormatAmount(req.Amount), epusdtFormatAmount(req.ActualAmount), req.Token, req.Status, c.ClientIP()))
		_, _ = c.Writer.WriteString("fail")
		return
	}

	// 只处理支付成功（status == 2）
	if req.Status != 2 {
		logger.LogInfo(c.Request.Context(), fmt.Sprintf("Epusdt webhook 忽略非成功状态 order_id=%q status=%d", req.OrderId, req.Status))
		if req.Status == 3 {
			_ = model.UpdatePendingTopUpStatus(req.OrderId, model.PaymentProviderEpusdt, common.TopUpStatusExpired)
		}
		_, _ = c.Writer.WriteString("ok")
		return
	}

	LockOrder(req.OrderId)
	defer UnlockOrder(req.OrderId)

	topUp := model.GetTopUpByTradeNo(req.OrderId)
	if topUp == nil {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("Epusdt webhook 订单不存在 order_id=%q client_ip=%s", req.OrderId, c.ClientIP()))
		_, _ = c.Writer.WriteString("fail")
		return
	}
	if topUp.PaymentProvider != model.PaymentProviderEpusdt {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("Epusdt webhook 支付渠道不匹配 order_id=%q provider=%s client_ip=%s", req.OrderId, topUp.PaymentProvider, c.ClientIP()))
		_, _ = c.Writer.WriteString("fail")
		return
	}
	if topUp.Status != common.TopUpStatusPending {
		logger.LogInfo(c.Request.Context(), fmt.Sprintf("Epusdt webhook 订单状态非 pending，忽略 order_id=%q status=%s", req.OrderId, topUp.Status))
		_, _ = c.Writer.WriteString("ok")
		return
	}
	expectedRoute := setting.ResolveEpusdtPaymentRoute(topUp.PaymentMethod)
	if expectedRoute.Network == "" {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("Epusdt webhook 支付网络配置无效 order_id=%q payment_method=%q callback_token=%q client_ip=%s", req.OrderId, topUp.PaymentMethod, req.Token, c.ClientIP()))
		_, _ = c.Writer.WriteString("fail")
		return
	}

	topUp.Status = common.TopUpStatusSuccess
	if err := topUp.Update(); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Epusdt 更新充值订单失败 order_id=%q user_id=%d error=%q", req.OrderId, topUp.UserId, err.Error()))
		_, _ = c.Writer.WriteString("fail")
		return
	}

	dAmount := decimal.NewFromInt(int64(topUp.Amount))
	dQuotaPerUnit := decimal.NewFromFloat(common.QuotaPerUnit)
	quotaToAdd := int(dAmount.Mul(dQuotaPerUnit).IntPart())

	if err := model.IncreaseUserQuota(topUp.UserId, quotaToAdd, true); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Epusdt 增加用户配额失败 order_id=%q user_id=%d quota_to_add=%d error=%q", req.OrderId, topUp.UserId, quotaToAdd, err.Error()))
		_, _ = c.Writer.WriteString("fail")
		return
	}

	logger.LogInfo(c.Request.Context(), fmt.Sprintf("Epusdt 充值成功 order_id=%q user_id=%d quota_to_add=%d money=%.2f", req.OrderId, topUp.UserId, quotaToAdd, topUp.Money))
	model.RecordTopupLog(topUp.UserId, fmt.Sprintf("使用 Epusdt 充值成功，充值金额: %v，支付金额：%.4f", logger.LogQuota(quotaToAdd), topUp.Money), c.ClientIP(), topUp.PaymentMethod, model.PaymentProviderEpusdt)

	_, _ = c.Writer.WriteString("ok")
}
