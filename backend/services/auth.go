package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

var tokenMutex sync.Mutex

type Auth struct {
	HttpClient     http.Client
	Host           string
	Secret         string
	ClientId       string
	cachedToken    string
	expirationTime time.Time
}

type TokenResponse struct {
	Token     string `json:"token"`
	ExpiresIn int64  `json:"expiresIn"`
}

type TokenClaims struct {
	Id          string   `json:"id"`
	TenantId    string   `json:"tenantId"`
	Permissions []string `json:"permissions"`
	Roles       []string `json:"roles"`
	Expires     string   `json:"expires"`
}

func (a *Auth) getAuthToken() (string, error) {
	tokenMutex.Lock()
	defer tokenMutex.Unlock()

	// Check if the token is still valid
	if time.Now().Before(a.expirationTime) {
		return a.cachedToken, nil
	}

	authUrl := a.Host + "/auth/vendor"
	payload := map[string]string{
		"clientId": a.ClientId,
		"secret":   a.Secret,
	}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", authUrl, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.HttpClient.Do(req)
	if err != nil {
		log.Printf("error while sending auth request: %v\n", err)
		return "", err
	}
	defer resp.Body.Close()

	var tokenResponse TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResponse); err != nil {
		return "", err
	}

	// Cache the token and expiration time
	a.cachedToken = tokenResponse.Token
	a.expirationTime = time.Now().Add(time.Second * time.Duration(tokenResponse.ExpiresIn))

	return tokenResponse.Token, nil
}

func (a *Auth) FetchTokenPermissions(tokenId string) ([]string, error) {

	accessTokenUrl := a.Host + "/identity/resources/vendor-only/tenants/access-tokens/v1/" + tokenId

	req, err := http.NewRequest("GET", accessTokenUrl, nil)

	if err != nil {
		return nil, fmt.Errorf("error while fetching token permissions: %v", err.Error())
	}

	authToken, err := a.getAuthToken()

	if err != nil {
		return nil, fmt.Errorf("error while fetching token permissions: %v", err.Error())
	}

	req.Header.Add("Authorization", "Bearer "+authToken)

	resp, err := a.HttpClient.Do(req)

	if err != nil {
		return nil, fmt.Errorf("error while fetching token permissions: %v", err.Error())
	}

	defer resp.Body.Close()

	var tokenClaims TokenClaims
	if err := json.NewDecoder(resp.Body).Decode(&tokenClaims); err != nil {
		return nil, fmt.Errorf("error while decoding token permissions: %v", err.Error())
	}

	return tokenClaims.Permissions, nil
}
