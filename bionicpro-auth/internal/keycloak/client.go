package keycloak

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
)

type Client struct {
	baseURL    string
	realm      string
	clientID   string
	clientSec  string
	httpClient *http.Client
}

func NewClient(keycloakBaseURL, realm, clientID, clientSecret string) *Client {
	return &Client{
		baseURL:   strings.TrimSuffix(keycloakBaseURL, "/"),
		realm:     realm,
		clientID:  clientID,
		clientSec: clientSecret,
		httpClient: &http.Client{},
	}
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

const kcLogPrefix = "[bionicpro-auth][keycloak]"

func (c *Client) ExchangeCode(ctx context.Context, code, redirectURI string) (*TokenResponse, error) {
	tokenURL := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token", c.baseURL, c.realm)
	log.Printf("%s POST %s grant_type=authorization_code redirect_uri=%s", kcLogPrefix, tokenURL, redirectURI)
	data := url.Values{
		"grant_type":   {"authorization_code"},
		"code":         {code},
		"redirect_uri": {redirectURI},
		"client_id":    {c.clientID},
		"client_secret": {c.clientSec},
	}
	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		log.Printf("%s exchange request error: %v", kcLogPrefix, err)
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Printf("%s exchange Do error: %v", kcLogPrefix, err)
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		log.Printf("%s exchange %d body=%s", kcLogPrefix, resp.StatusCode, string(body))
		return nil, fmt.Errorf("token exchange failed: %s", string(body))
	}
	log.Printf("%s exchange 200 OK", kcLogPrefix)
	var tr TokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return nil, err
	}
	return &tr, nil
}

func (c *Client) RefreshToken(ctx context.Context, refreshToken string) (*TokenResponse, error) {
	tokenURL := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token", c.baseURL, c.realm)
	log.Printf("%s POST %s grant_type=refresh_token", kcLogPrefix, tokenURL)
	data := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {c.clientID},
		"client_secret": {c.clientSec},
	}
	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		log.Printf("%s refresh request error: %v", kcLogPrefix, err)
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Printf("%s refresh Do error: %v", kcLogPrefix, err)
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		log.Printf("%s refresh %d body=%s", kcLogPrefix, resp.StatusCode, string(body))
		return nil, fmt.Errorf("token refresh failed: %s", string(body))
	}
	log.Printf("%s refresh 200 OK", kcLogPrefix)
	var tr TokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return nil, err
	}
	return &tr, nil
}
