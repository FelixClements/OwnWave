package streaming

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

const (
	targetLoudness = -14.0
	maxGainDb      = 20.0
	minGainDb      = -20.0
	minBitrateKbps = 32
	maxBitrateKbps = 320
)

func NormalizeBitrate(input, defaultRate string) string {
	input = strings.ToLower(strings.TrimSpace(input))
	input = strings.TrimSuffix(input, "k")
	kbps, err := strconv.Atoi(input)
	if err != nil || kbps < minBitrateKbps || kbps > maxBitrateKbps {
		return defaultRate
	}
	return fmt.Sprintf("%dk", kbps)
}

func VolumeGainDb(loudness *float64, normalize bool) float64 {
	if !normalize || loudness == nil || *loudness == 0 || math.IsNaN(*loudness) || math.IsInf(*loudness, 0) {
		return 0
	}
	gain := targetLoudness - *loudness
	if math.IsNaN(gain) || math.IsInf(gain, 0) {
		return 0
	}
	if gain > maxGainDb {
		return maxGainDb
	}
	if gain < minGainDb {
		return minGainDb
	}
	return gain
}
