package reporttoken

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Payload struct {
	UserID      string
	PeriodStart string
	PeriodEnd   string
	ExpiresAt   int64
}

func Sign(secret string, p Payload, ttlMinutes int) string {
	exp := time.Now().Add(time.Duration(ttlMinutes) * time.Minute).Unix()
	p.ExpiresAt = exp
	payload := fmt.Sprintf("%s|%s|%s|%d", p.UserID, p.PeriodStart, p.PeriodEnd, exp)
	b64 := base64.URLEncoding.EncodeToString([]byte(payload))
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	sig := hex.EncodeToString(mac.Sum(nil))
	return b64 + "." + sig
}

func Verify(secret, token string) (*Payload, error) {
	idx := strings.LastIndex(token, ".")
	if idx <= 0 {
		return nil, fmt.Errorf("invalid token format")
	}
	b64, sigHex := token[:idx], token[idx+1:]
	payloadBytes, err := base64.URLEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("invalid token base64: %w", err)
	}
	payloadStr := string(payloadBytes)
	sig, err := hex.DecodeString(sigHex)
	if err != nil {
		return nil, fmt.Errorf("invalid token signature: %w", err)
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payloadStr))
	if !hmac.Equal(mac.Sum(nil), sig) {
		return nil, fmt.Errorf("invalid token signature")
	}
	parts := strings.SplitN(payloadStr, "|", 4)
	if len(parts) != 4 {
		return nil, fmt.Errorf("invalid token payload")
	}
	exp, _ := strconv.ParseInt(parts[3], 10, 64)
	if time.Now().Unix() > exp {
		return nil, fmt.Errorf("token expired")
	}
	return &Payload{
		UserID:      parts[0],
		PeriodStart: parts[1],
		PeriodEnd:   parts[2],
		ExpiresAt:   exp,
	}, nil
}
