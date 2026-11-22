package stations

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// The web and Android apps read these keys; keep them as they are.
func TestQueueTrackJSONKeys(t *testing.T) {
	artist, n, f := "A", 1, 1.5
	b, err := json.Marshal(QueueTrack{ID: "id", Title: "t", Artist: &artist, Album: &artist, TrackNumber: &n,
		DurationSeconds: &f, SampleRate: &n, Channels: &n, Loudness: &f, Liked: true})
	require.NoError(t, err)
	require.JSONEq(t, `{"id":"id","title":"t","artist":"A","album":"A","track_number":1,"duration_seconds":1.5,
		"sample_rate":1,"channels":1,"loudness":1.5,"bpm":0,"key":"","energy":0,"valence":0,
		"outro_start_seconds":0,"ideal_crossfade_seconds":0,"intro_start_seconds":0,"outro_end_seconds":0,
		"position":0,"liked":true}`, string(b))

	b, err = json.Marshal(QueueTrack{ID: "id", Title: "t"})
	require.NoError(t, err)
	require.JSONEq(t, `{"id":"id","title":"t","bpm":0,"key":"","energy":0,"valence":0,
		"outro_start_seconds":0,"ideal_crossfade_seconds":0,"intro_start_seconds":0,"outro_end_seconds":0,
		"position":0,"liked":false}`, string(b))
}
