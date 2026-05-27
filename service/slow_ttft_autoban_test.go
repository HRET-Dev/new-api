package service

import "testing"

func resetSlowTTFTTestState() {
	slowTTFTMu.Lock()
	slowTTFTStates = map[slowTTFTKey]slowTTFTState{}
	slowTTFTMu.Unlock()
}

func TestRecordSlowTTFTSampleRequiresConsecutiveSlowSamples(t *testing.T) {
	resetSlowTTFTTestState()
	key := slowTTFTKey{channelID: 42, usingKey: "key-a"}
	const thresholdMs int64 = 30_000
	const consecutiveLimit = 3

	count, shouldDisable := recordSlowTTFTSample(key, 31_000, thresholdMs, consecutiveLimit)
	if count != 1 || shouldDisable {
		t.Fatalf("first slow sample = (%d, %t), want (1, false)", count, shouldDisable)
	}

	count, shouldDisable = recordSlowTTFTSample(key, 31_000, thresholdMs, consecutiveLimit)
	if count != 2 || shouldDisable {
		t.Fatalf("second slow sample = (%d, %t), want (2, false)", count, shouldDisable)
	}

	count, shouldDisable = recordSlowTTFTSample(key, 31_000, thresholdMs, consecutiveLimit)
	if count != consecutiveLimit || !shouldDisable {
		t.Fatalf("third slow sample = (%d, %t), want (%d, true)", count, shouldDisable, consecutiveLimit)
	}

	slowTTFTMu.Lock()
	_, exists := slowTTFTStates[key]
	slowTTFTMu.Unlock()
	if exists {
		t.Fatal("slow TTFT state should reset after disable threshold is reached")
	}
}

func TestRecordSlowTTFTSampleNormalResponseResetsCount(t *testing.T) {
	resetSlowTTFTTestState()
	key := slowTTFTKey{channelID: 42, usingKey: "key-a"}
	const thresholdMs int64 = 30_000
	const consecutiveLimit = 3

	count, shouldDisable := recordSlowTTFTSample(key, 31_000, thresholdMs, consecutiveLimit)
	if count != 1 || shouldDisable {
		t.Fatalf("first slow sample = (%d, %t), want (1, false)", count, shouldDisable)
	}

	count, shouldDisable = recordSlowTTFTSample(key, thresholdMs, thresholdMs, consecutiveLimit)
	if count != 0 || shouldDisable {
		t.Fatalf("normal sample = (%d, %t), want (0, false)", count, shouldDisable)
	}

	count, shouldDisable = recordSlowTTFTSample(key, 31_000, thresholdMs, consecutiveLimit)
	if count != 1 || shouldDisable {
		t.Fatalf("slow sample after reset = (%d, %t), want (1, false)", count, shouldDisable)
	}
}

func TestRecordSlowTTFTSampleUsesConfiguredConsecutiveLimit(t *testing.T) {
	resetSlowTTFTTestState()
	key := slowTTFTKey{channelID: 42, usingKey: "key-a"}
	const thresholdMs int64 = 30_000
	const consecutiveLimit = 2

	count, shouldDisable := recordSlowTTFTSample(key, 31_000, thresholdMs, consecutiveLimit)
	if count != 1 || shouldDisable {
		t.Fatalf("first slow sample = (%d, %t), want (1, false)", count, shouldDisable)
	}

	count, shouldDisable = recordSlowTTFTSample(key, 31_000, thresholdMs, consecutiveLimit)
	if count != consecutiveLimit || !shouldDisable {
		t.Fatalf("second slow sample = (%d, %t), want (%d, true)", count, shouldDisable, consecutiveLimit)
	}
}
