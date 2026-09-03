package playback

type Track struct {
	ID              string   `json:"id"`
	Title           string   `json:"title"`
	Artist          *string  `json:"artist,omitempty"`
	Album           *string  `json:"album,omitempty"`
	Path            string   `json:"-"`
	TrackNumber     *int     `json:"track_number,omitempty"`
	DurationSeconds *float64 `json:"duration_seconds,omitempty"`
	SampleRate      *int     `json:"sample_rate,omitempty"`
	Channels        *int     `json:"channels,omitempty"`
	Loudness        *float64 `json:"loudness,omitempty"`
}

type TrackWithFeatures struct {
	Track
	BPM                   float64 `json:"bpm"`
	Key                   string  `json:"key"`
	Energy                float64 `json:"energy"`
	Valence               float64 `json:"valence"`
	OutroStartSeconds     float64 `json:"outro_start_seconds"`
	IdealCrossfadeSeconds float64 `json:"ideal_crossfade_seconds"`
	IntroStartSeconds     float64 `json:"intro_start_seconds"`
	OutroEndSeconds       float64 `json:"outro_end_seconds"`
	Position              int     `json:"position"`
	Liked                 bool    `json:"liked"`
}
