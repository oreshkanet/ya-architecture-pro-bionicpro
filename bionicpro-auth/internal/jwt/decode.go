package jwt

import (
	"encoding/base64"
	"encoding/json"
	"strings"
)

// UsernameFromToken извлекает claim "preferred_username" (Keycloak username) из access token JWT
func UsernameFromToken(accessToken string) (string, error) {
	parts := strings.Split(accessToken, ".")
	if len(parts) != 3 {
		return "", nil
	}
	payload := parts[1]
	buf, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return "", err
	}
	var claims struct {
		PreferredUsername string `json:"preferred_username"`
		Sub               string `json:"sub"`
	}
	if err := json.Unmarshal(buf, &claims); err != nil {
		return "", err
	}
	if claims.PreferredUsername != "" {
		return claims.PreferredUsername, nil
	}
	return claims.Sub, nil
}
