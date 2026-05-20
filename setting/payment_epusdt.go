package setting

import "strings"

var (
	// EpusdtEnabled 是否启用 Epusdt 支付
	EpusdtEnabled bool
	// EpusdtApiUrl Epusdt 服务地址，例如 https://epusdt.example.com
	EpusdtApiUrl string
	// EpusdtPid Epusdt 商户 PID
	EpusdtPid string
	// EpusdtSecretKey Epusdt 商户密钥
	EpusdtSecretKey string
	// EpusdtCurrency Epusdt 创建订单使用的法币类型
	EpusdtCurrency string = "cny"
	// EpusdtTradeTypes 允许作为付款方式 type 使用的 Epusdt 网络类型列表，推荐格式为 network.default_token
	EpusdtTradeTypes string = "tron.usdt\nbsc.usdt\nethereum.usdt"
)

// EpusdtPaymentRoute 表示 Epusdt 上游创建订单所需的币种和链路。
type EpusdtPaymentRoute struct {
	Token   string
	Network string
}

// GetEpusdtSecretKey 返回 Epusdt 签名密钥。
func GetEpusdtSecretKey() string {
	return strings.TrimSpace(EpusdtSecretKey)
}

// GetEpusdtCurrency 返回 Epusdt 法币代码。
func GetEpusdtCurrency() string {
	currency := strings.ToLower(strings.TrimSpace(EpusdtCurrency))
	switch currency {
	case "cny", "usd", "eur", "gbp", "jpy":
		return currency
	}
	return "cny"
}

// GetEpusdtTradeTypes 返回已配置的 Epusdt 交易类型列表。
func GetEpusdtTradeTypes() []string {
	if strings.TrimSpace(EpusdtTradeTypes) == "" {
		return []string{"tron.usdt", "bsc.usdt", "ethereum.usdt"}
	}
	types := parseEpusdtTradeTypes(EpusdtTradeTypes)
	if len(types) > 0 {
		return types
	}
	return []string{}
}

// IsEpusdtTradeType 判断付款方式 type 是否为 Epusdt 交易类型。
func IsEpusdtTradeType(tradeType string) bool {
	tradeType = normalizeEpusdtTradeType(tradeType)
	if tradeType == "" {
		return false
	}
	for _, item := range GetEpusdtTradeTypes() {
		if normalizeEpusdtTradeType(item) == tradeType {
			return true
		}
	}
	return false
}

// ResolveEpusdtPaymentRoute 根据本地支付方式 type 解析 Epusdt 上游网络和默认币种。
//
// 支持推荐格式 network.default_token，例如：
//   - tron.usdt -> network=tron, token=usdt
//   - bsc.usdt -> network=bsc, token=usdt
//   - bsc.usdc -> network=bsc, token=usdc
func ResolveEpusdtPaymentRoute(tradeType string) EpusdtPaymentRoute {
	network, token, ok := parseEpusdtTradeTypeRoute(tradeType)
	if ok {
		return EpusdtPaymentRoute{
			Token:   token,
			Network: network,
		}
	}
	return EpusdtPaymentRoute{}
}

func parseEpusdtTradeTypes(value string) []string {
	normalized := strings.NewReplacer(",", "\n", ";", "\n", "\r", "\n").Replace(value)
	seen := make(map[string]bool)
	result := make([]string, 0)
	for _, line := range strings.Split(normalized, "\n") {
		item := normalizeEpusdtTradeType(line)
		if item == "" || seen[item] {
			continue
		}
		if _, _, ok := parseEpusdtTradeTypeRoute(item); !ok {
			continue
		}
		seen[item] = true
		result = append(result, item)
	}
	return result
}

func normalizeEpusdtTradeType(tradeType string) string {
	return strings.ToLower(strings.TrimSpace(tradeType))
}

func parseEpusdtTradeTypeRoute(tradeType string) (string, string, bool) {
	tradeType = normalizeEpusdtTradeType(tradeType)
	if tradeType == "" {
		return "", "", false
	}

	parts := strings.FieldsFunc(tradeType, func(r rune) bool {
		return r == '.' || r == ':' || r == '_'
	})
	if len(parts) != 2 {
		return "", "", false
	}

	network := normalizeEpusdtNetwork(parts[0])
	token := normalizeEpusdtToken(parts[1])
	if network == "" || token == "" {
		return "", "", false
	}
	return network, token, true
}

func normalizeEpusdtNetwork(network string) string {
	network = strings.ToLower(strings.TrimSpace(network))
	switch network {
	case "tron", "bsc", "ethereum", "solana":
		return network
	}
	return ""
}

func normalizeEpusdtToken(token string) string {
	token = strings.ToLower(strings.TrimSpace(token))
	if token == "" {
		return ""
	}
	for _, r := range token {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return ""
		}
	}
	return token
}
