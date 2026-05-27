package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/bytedance/gopkg/util/gopool"
)

const (
	slowTTFTRedisKeyPrefix = "slow_ttft_autoban:"
	slowTTFTCounterTTL     = 24 * time.Hour
	slowTTFTRedisTimeout   = 2 * time.Second
)

const slowTTFTIncrementScript = `
local count = redis.call("INCR", KEYS[1])
redis.call("EXPIRE", KEYS[1], ARGV[1])
if count >= tonumber(ARGV[2]) then
	redis.call("DEL", KEYS[1])
	return {count, 1}
end
return {count, 0}
`

type slowTTFTKey struct {
	channelID int
	usingKey  string
}

type slowTTFTState struct {
	count      int
	lastSeenAt time.Time
}

var (
	slowTTFTMu     sync.Mutex
	slowTTFTStates = map[slowTTFTKey]slowTTFTState{}
)

type slowTTFTSample struct {
	channelError     types.ChannelError
	isStream         bool
	hasFirstResponse bool
	retryIndex       int
	startTime        time.Time
	firstResponse    time.Time
	promptTokens     int
	thresholdSeconds float64
	consecutiveLimit int
}

func slowTTFTRedisEnabled() bool {
	return common.RedisEnabled && common.RDB != nil
}

func slowTTFTRedisKey(key slowTTFTKey) string {
	keyHash := sha256.Sum256([]byte(key.usingKey))
	return fmt.Sprintf("%s%d:%s", slowTTFTRedisKeyPrefix, key.channelID, hex.EncodeToString(keyHash[:8]))
}

func resetSlowTTFTMemoryCount(key slowTTFTKey) {
	slowTTFTMu.Lock()
	delete(slowTTFTStates, key)
	slowTTFTMu.Unlock()
}

func resetSlowTTFTCount(key slowTTFTKey) {
	resetSlowTTFTMemoryCount(key)
	if !slowTTFTRedisEnabled() {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), slowTTFTRedisTimeout)
	defer cancel()
	if err := common.RDB.Del(ctx, slowTTFTRedisKey(key)).Err(); err != nil {
		logger.LogWarn(context.Background(), fmt.Sprintf("slow TTFT Redis counter reset failed: channel_id=%d err=%v", key.channelID, err))
	}
}

func recordSlowTTFTMemorySlowSample(key slowTTFTKey, consecutiveLimit int) (int, bool) {
	slowTTFTMu.Lock()
	defer slowTTFTMu.Unlock()

	state := slowTTFTStates[key]
	state.count++
	state.lastSeenAt = time.Now()
	if state.count >= consecutiveLimit {
		delete(slowTTFTStates, key)
		return state.count, true
	}

	slowTTFTStates[key] = state
	return state.count, false
}

func redisScriptInt(value any) (int64, bool) {
	switch typed := value.(type) {
	case int64:
		return typed, true
	case int:
		return int64(typed), true
	default:
		return 0, false
	}
}

func recordSlowTTFTRedisSlowSample(key slowTTFTKey, consecutiveLimit int) (int, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), slowTTFTRedisTimeout)
	defer cancel()

	result, err := common.RDB.Eval(
		ctx,
		slowTTFTIncrementScript,
		[]string{slowTTFTRedisKey(key)},
		int64(slowTTFTCounterTTL.Seconds()),
		int64(consecutiveLimit),
	).Result()
	if err != nil {
		return 0, false, err
	}

	values, ok := result.([]interface{})
	if !ok || len(values) != 2 {
		return 0, false, fmt.Errorf("unexpected Redis script result: %T", result)
	}
	count, ok := redisScriptInt(values[0])
	if !ok {
		return 0, false, fmt.Errorf("unexpected Redis count type: %T", values[0])
	}
	shouldDisable, ok := redisScriptInt(values[1])
	if !ok {
		return 0, false, fmt.Errorf("unexpected Redis disable flag type: %T", values[1])
	}
	return int(count), shouldDisable == 1, nil
}

func recordSlowTTFTSample(key slowTTFTKey, ttftMs int64, thresholdMs int64, consecutiveLimit int) (int, bool) {
	if ttftMs <= thresholdMs {
		resetSlowTTFTCount(key)
		return 0, false
	}

	if slowTTFTRedisEnabled() {
		count, shouldDisable, err := recordSlowTTFTRedisSlowSample(key, consecutiveLimit)
		if err == nil {
			return count, shouldDisable
		}
		logger.LogWarn(context.Background(), fmt.Sprintf("slow TTFT Redis counter increment failed, falling back to memory: channel_id=%d err=%v", key.channelID, err))
	}
	return recordSlowTTFTMemorySlowSample(key, consecutiveLimit)
}

// QueueSlowTTFTAutoDisable copies the timing data needed for the slow-response
// strategy and evaluates it asynchronously so the client response path is not
// delayed by counter updates, database writes, or notifications.
func QueueSlowTTFTAutoDisable(info *relaycommon.RelayInfo, channelError types.ChannelError) {
	if info == nil {
		return
	}
	setting := operation_setting.GetMonitorSetting()
	if !setting.AutoDisableSlowResponseEnabled || !channelError.AutoBan {
		return
	}
	if setting.AutoDisableSlowResponseSeconds <= 0 || setting.AutoDisableSlowResponseCount <= 0 {
		return
	}
	sample := slowTTFTSample{
		channelError:     channelError,
		isStream:         info.IsStream,
		hasFirstResponse: info.HasSendResponse(),
		retryIndex:       info.RetryIndex,
		startTime:        info.StartTime,
		firstResponse:    info.FirstResponseTime,
		promptTokens:     info.GetEstimatePromptTokens(),
		thresholdSeconds: setting.AutoDisableSlowResponseSeconds,
		consecutiveLimit: setting.AutoDisableSlowResponseCount,
	}
	gopool.Go(func() {
		handleSlowTTFTAutoDisable(sample)
	})
}

func handleSlowTTFTAutoDisable(sample slowTTFTSample) {
	if !sample.isStream || !sample.hasFirstResponse {
		return
	}
	if sample.retryIndex > 0 {
		return
	}

	ttftMs := sample.firstResponse.Sub(sample.startTime).Milliseconds()
	thresholdMs := int64(sample.thresholdSeconds * 1000)
	key := slowTTFTKey{
		channelID: sample.channelError.ChannelId,
		usingKey:  sample.channelError.UsingKey,
	}

	count, shouldDisable := recordSlowTTFTSample(key, ttftMs, thresholdMs, sample.consecutiveLimit)
	if count == 0 {
		return
	}
	logger.LogWarn(context.Background(), fmt.Sprintf("channel #%d slow TTFT detected: ttft=%.2fs threshold=%.2fs count=%d/%d prompt_tokens=%d",
		sample.channelError.ChannelId,
		float64(ttftMs)/1000.0,
		sample.thresholdSeconds,
		count,
		sample.consecutiveLimit,
		sample.promptTokens,
	))
	if !shouldDisable {
		return
	}

	reason := fmt.Sprintf("TTFT %.2fs exceeded threshold %.2fs for %d consecutive streaming requests (prompt_tokens=%d)",
		float64(ttftMs)/1000.0,
		sample.thresholdSeconds,
		sample.consecutiveLimit,
		sample.promptTokens,
	)
	DisableChannel(sample.channelError, reason)
}
