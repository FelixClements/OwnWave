package streaming

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	targetLoudness = -14.0
	maxGainDb      = 20.0
	minGainDb      = -20.0
)

func NormalizeBitrate(input, defaultRate string) string {
	input = strings.ToLower(strings.TrimSpace(input))
	input = strings.TrimSuffix(input, "k")
	kbps, err := strconv.Atoi(input)
	if err != nil || kbps <= 0 {
		return defaultRate
	}
	return fmt.Sprintf("%dk", kbps)
}

func VolumeGainDb(loudness *float64, normalize bool) float64 {
	if !normalize || loudness == nil || *loudness == 0 {
		return 0
	}
	gain := targetLoudness - *loudness
	if gain > maxGainDb {
		return maxGainDb
	}
	if gain < minGainDb {
		return minGainDb
	}
	return gain
}
