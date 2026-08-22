package streamauth

import (
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// StreamTokens signs and validates short-lived JWT capability URLs for audio streaming.
type StreamTokens struct {
	secret []byte
}

func New(secret []byte) *StreamTokens {
	return &StreamTokens{secret: secret}
}

func (t *StreamTokens) SignTrack(trackID, format string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"track_id": trackID,
		"format":   format,
		"exp":      time.Now().Add(10 * time.Minute).Unix(),
	})
	return token.SignedString(t.secret)
}

func (t *StreamTokens) ValidateTrack(tokenString string) (trackID, format string, err error) {
	claims, err := t.parseClaims(tokenString)
	if err != nil {
		return "", "", err
	}
	return getString(claims, "track_id"), getString(claims, "format"), nil
}

func (t *StreamTokens) SignStation(stationID, format string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"station_id": stationID,
		"format":     format,
		"exp":        time.Now().Add(10 * time.Minute).Unix(),
	})
	return token.SignedString(t.secret)
}

func (t *StreamTokens) ValidateStation(tokenString string) (stationID, format string, err error) {
	claims, err := t.parseClaims(tokenString)
	if err != nil {
		return "", "", err
	}
	return getString(claims, "station_id"), getString(claims, "format"), nil
}

func (t *StreamTokens) parseClaims(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(tok *jwt.Token) (interface{}, error) {
		if _, ok := tok.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return t.secret, nil
	})
	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid claims")
	}
	return claims, nil
}

func getString(claims jwt.MapClaims, key string) string {
	if v, ok := claims[key].(string); ok {
		return v
	}
	if v, ok := claims[key].(float64); ok {
		return strconv.FormatFloat(v, 'f', -1, 64)
	}
	return ""
}
