package setting

import "testing"

func TestResolveEpusdtPaymentRoute(t *testing.T) {
	tests := []struct {
		name      string
		tradeType string
		token     string
		network   string
	}{
		{
			name:      "Epusdt Tron",
			tradeType: "tron.usdt",
			token:     "usdt",
			network:   "tron",
		},
		{
			name:      "Epusdt BSC",
			tradeType: "bsc.usdt",
			token:     "usdt",
			network:   "bsc",
		},
		{
			name:      "Epusdt BSC USDC default token",
			tradeType: "bsc.usdc",
			token:     "usdc",
			network:   "bsc",
		},
		{
			name:      "Epusdt Ethereum",
			tradeType: "ethereum.usdt",
			token:     "usdt",
			network:   "ethereum",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := ResolveEpusdtPaymentRoute(tt.tradeType)
			if route.Token != tt.token || route.Network != tt.network {
				t.Fatalf(
					"expected token=%s network=%s, got token=%s network=%s",
					tt.token,
					tt.network,
					route.Token,
					route.Network,
				)
			}
		})
	}
}

func TestResolveEpusdtPaymentRouteInvalid(t *testing.T) {
	tests := []string{"custom-wallet", "usdt.bsc", "usdc.bsc", "epusdt.bsc.extra", "bsc.usdt.extra", "unknown.usdt"}
	for _, tradeType := range tests {
		route := ResolveEpusdtPaymentRoute(tradeType)
		if route.Token != "" || route.Network != "" {
			t.Fatalf("expected empty route for %s, got token=%s network=%s", tradeType, route.Token, route.Network)
		}
	}
}
