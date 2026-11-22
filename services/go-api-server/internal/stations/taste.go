package stations

// Taste: the user's bans, likes and hides. The predicates below are on
// `tracks t` where t is a song's best copy, as Definition.where yields
// them, so t.id is the song's id (songs.song_id). Feedback and hides on any
// copy of the song count. stations writes station_hidden_tracks; it only
// reads track_feedback, which another module writes (ADR 0005).

// notBanned is an SQL predicate on a best copy `tracks t` that drops the
// songs the user (at placeholder userArg) banned.
func notBanned(userArg string) string {
	return `NOT EXISTS (
		SELECT 1 FROM track_feedback b JOIN songs bs ON bs.track_id = b.track_id
		WHERE b.user_id = ` + userArg + `::uuid AND b.feedback = 'ban' AND bs.song_id = t.id)`
}

// liked is an SQL predicate on a best copy `tracks t`: the user (at
// placeholder userArg) liked this song. It is an uncorrelated IN, which
// Postgres hashes once; a correlated EXISTS in the queue's select list
// raised its cost estimate enough to JIT-compile the query (#111).
func liked(userArg string) string {
	return `t.id IN (
		SELECT ls.song_id FROM track_feedback l JOIN songs ls ON ls.track_id = l.track_id
		WHERE l.user_id = ` + userArg + `::uuid AND l.feedback = 'like')`
}

// notHidden is an SQL predicate on a best copy `tracks t` that drops the
// songs the user hid on the station (at placeholders userArg and stationArg).
func notHidden(userArg, stationArg string) string {
	return `NOT EXISTS (
		SELECT 1 FROM station_hidden_tracks h JOIN songs hs ON hs.track_id = h.track_id
		WHERE h.user_id = ` + userArg + `::uuid AND h.station_id = ` + stationArg + `::uuid AND hs.song_id = t.id)`
}
