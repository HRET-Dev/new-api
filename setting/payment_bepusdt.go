package setting

import "strings"

var (
	// BEPUsdtEnabled 是否启用 BEPUsdt 支付
	BEPUsdtEnabled bool
	// BEPUsdtApiUrl BEPUsdt 服务地址，例如 https://bepusdt.example.com
	BEPUsdtApiUrl string
	// BEPUsdtToken API 认证 token，用于签名计算
	BEPUsdtToken string
	// BEPUsdtFiatCurrency BEPUsdt 创建订单使用的法币类型。
	BEPUsdtFiatCurrency string = "CNY"
	// BEPUsdtTradeType 交易类型：usdt.bep20 / usdt.aptos / usdt.arbitrum
	BEPUsdtTradeType string = "usdt.bep20"
	// BEPUsdtTradeTypes 允许作为付款方式 type 使用的 BEPUsdt 交易类型列表
	BEPUsdtTradeTypes string = "usdt.bep20\nusdt.aptos\nusdt.arbitrum"
)

// GetBEPUsdtTradeTypes 返回已配置的 BEPUsdt 交易类型列表。
func GetBEPUsdtTradeTypes() []string {
	types := parseBEPUsdtTradeTypes(BEPUsdtTradeTypes)
	if len(types) > 0 {
		return types
	}
	return []string{"usdt.bep20", "usdt.aptos", "usdt.arbitrum"}
}

// IsBEPUsdtTradeType 判断付款方式 type 是否为 BEPUsdt 交易类型。
func IsBEPUsdtTradeType(tradeType string) bool {
	tradeType = strings.TrimSpace(tradeType)
	if tradeType == "" {
		return false
	}
	for _, item := range GetBEPUsdtTradeTypes() {
		if item == tradeType {
			return true
		}
	}
	return false
}

func parseBEPUsdtTradeTypes(value string) []string {
	normalized := strings.NewReplacer(",", "\n", ";", "\n", "\r", "\n").Replace(value)
	seen := make(map[string]bool)
	result := make([]string, 0)
	for _, line := range strings.Split(normalized, "\n") {
		item := strings.TrimSpace(line)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		result = append(result, item)
	}
	return result
}
