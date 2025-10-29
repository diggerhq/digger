package auth

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
)

type Signer struct {
	priv       ed25519.PrivateKey
	pub        ed25519.PublicKey
	kid        string
	accessTTL  time.Duration
	refreshTTL time.Duration
	issuer     string
}

func NewSignerFromEnv() (*Signer, error) {
	// Key loading
	pemPath := getenv("OPENTACO_TOKENS_PRIVATE_KEY_PEM_PATH", "")
	var priv ed25519.PrivateKey
	var pub ed25519.PublicKey
	var err error
	if pemPath != "" {
		log.Printf("JWT: Loading Ed25519 private key from: %s", pemPath)
		priv, pub, err = loadEd25519FromPEM(pemPath)
		if err != nil {
			return nil, fmt.Errorf("load ed25519 pem: %w", err)
		}
		log.Printf("JWT: Successfully loaded Ed25519 key from file")
	} else {
		log.Printf("JWT: No private key file specified, generating ephemeral key for development")
		pub, priv, err = ed25519.GenerateKey(nil)
		if err != nil {
			return nil, fmt.Errorf("generate ed25519: %w", err)
		}
		log.Printf("JWT: Generated ephemeral Ed25519 key")
	}
	
	kid := getenv("OPENTACO_TOKENS_KID", "k1")
	access := getenv("OPENTACO_TOKENS_ACCESS_TTL", "1h")
	refresh := getenv("OPENTACO_TOKENS_REFRESH_TTL", "720h")
	accessTTL, err := time.ParseDuration(access)
	if err != nil {
		accessTTL = time.Hour
	}
	refreshTTL, err := time.ParseDuration(refresh)
	if err != nil {
		refreshTTL = 30 * 24 * time.Hour
	}
	issuer := getenv("OPENTACO_PUBLIC_BASE_URL", "http://localhost:8080")

	log.Printf("JWT: Configuration loaded - KID: %s, Access TTL: %v, Refresh TTL: %v, Issuer: %s", 
		kid, accessTTL, refreshTTL, issuer)

	return &Signer{priv: priv, pub: pub, kid: kid, accessTTL: accessTTL, refreshTTL: refreshTTL, issuer: issuer}, nil
}

func loadEd25519FromPEM(path string) (ed25519.PrivateKey, ed25519.PublicKey, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	block, _ := pem.Decode(b)
	if block == nil {
		return nil, nil, errors.New("invalid PEM")
	}
	// Try PKCS8 first
	if k, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		if pk, ok := k.(ed25519.PrivateKey); ok {
			pub := pk.Public().(ed25519.PublicKey)
			return pk, pub, nil
		}
	}
	// Try raw ed25519 private key
	if pk, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		if k, ok := pk.(ed25519.PrivateKey); ok {
			return k, k.Public().(ed25519.PublicKey), nil
		}
	}
	return nil, nil, errors.New("unsupported private key format")
}

type accessClaims struct {
	Roles  []string `json:"roles,omitempty"`
	Groups []string `json:"groups,omitempty"`
	Scopes []string `json:"scopes,omitempty"`
	Org    string   `json:"org_uuid,omitempty"`
	Email  string   `json:"email,omitempty"`
	jwt.RegisteredClaims
}

type refreshClaims struct {
	RID string `json:"rid"`
	jwt.RegisteredClaims
}

type oauthCodeClaims struct {
	ClientID      string   `json:"client_id"`
	RedirectURI   string   `json:"redirect_uri"`
	Email         string   `json:"email,omitempty"`
	Groups        []string `json:"groups,omitempty"`
	Org           string   `json:"org_uuid,omitempty"`
	CodeChallenge string   `json:"code_challenge"`
	jwt.RegisteredClaims
}

func (s *Signer) MintAccess(sub string, roles, groups, scopes []string) (string, time.Time, error) {
	return s.MintAccessWithEmail(sub, "", roles, groups, scopes)
}

func (s *Signer) MintAccessWithEmail(sub, email string, roles, groups, scopes []string) (string, time.Time, error) {
	return s.MintAccessWithOrg(sub, email, roles, groups, scopes, "")
}

func (s *Signer) MintAccessWithOrg(sub, email string, roles, groups, scopes []string, org string) (string, time.Time, error) {
	now := time.Now()
	exp := now.Add(s.accessTTL)
	claims := accessClaims{
		Roles:  roles,
		Groups: groups,
		Scopes: scopes,
		Org:    org,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.issuer,
			Subject:   sub,
			Audience:  []string{"api", "s3"},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	token.Header["kid"] = s.kid
	tokenStr, err := token.SignedString(s.priv)
	return tokenStr, exp, err
}

// MintAccessWithEmailAndTTL creates an access token with a custom TTL
func (s *Signer) MintAccessWithEmailAndTTL(sub, email string, roles, groups, scopes []string, ttl time.Duration) (string, time.Time, error) {
    return s.MintAccessWithOrgAndTTL(sub, email, roles, groups, scopes, "", ttl)
}

func (s *Signer) MintAccessWithOrgAndTTL(sub, email string, roles, groups, scopes []string, org string, ttl time.Duration) (string, time.Time, error) {
    now := time.Now()
    exp := now.Add(ttl)
    claims := accessClaims{
        Roles:  roles,
        Groups: groups,
        Scopes: scopes,
        Org:    org,
        Email:  email,
        RegisteredClaims: jwt.RegisteredClaims{
            Issuer:    s.issuer,
            Subject:   sub,
            Audience:  []string{"api", "s3"},
            IssuedAt:  jwt.NewNumericDate(now),
            ExpiresAt: jwt.NewNumericDate(exp),
        },
    }
    token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
    token.Header["kid"] = s.kid
    tokenStr, err := token.SignedString(s.priv)
    return tokenStr, exp, err
}

func (s *Signer) MintRefresh(sub, rid string) (string, time.Time, error) {
	now := time.Now()
	exp := now.Add(s.refreshTTL)
	claims := refreshClaims{
		RID: rid,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.issuer,
			Subject:   sub,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	token.Header["kid"] = s.kid
	tokenStr, err := token.SignedString(s.priv)
	return tokenStr, exp, err
}

// MintOAuthCode creates a JWT authorization code for OAuth flows (5-minute expiry)
func (s *Signer) MintOAuthCode(sub, email, clientID, redirectURI, codeChallenge, org string, groups []string) (string, time.Time, error) {
	now := time.Now()
	exp := now.Add(5 * time.Minute)
	claims := oauthCodeClaims{
		ClientID:      clientID,
		RedirectURI:   redirectURI,
		Email:         email,
		Groups:        groups,
		Org:           org,
		CodeChallenge: codeChallenge,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.issuer,
			Subject:   sub,
			Audience:  []string{"oauth-authorization-code"},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	token.Header["kid"] = s.kid
	tokenStr, err := token.SignedString(s.priv)
	return tokenStr, exp, err
}

func (s *Signer) VerifyAccess(tokenStr string) (*accessClaims, error) {
	parser := jwt.NewParser(jwt.WithValidMethods([]string{jwt.SigningMethodEdDSA.Alg()}))
	var claims accessClaims
	_, err := parser.ParseWithClaims(tokenStr, &claims, func(t *jwt.Token) (interface{}, error) {
		return s.pub, nil
	})
	if err != nil {
		return nil, err
	}
	// Audience check is lenient: ensure at least one of api/s3
	if !containsAny(claims.RegisteredClaims.Audience, []string{"api", "s3"}) {
		return nil, errors.New("invalid audience")
	}
	return &claims, nil
}

func (s *Signer) VerifyRefresh(tokenStr string) (*refreshClaims, error) {
	parser := jwt.NewParser(jwt.WithValidMethods([]string{jwt.SigningMethodEdDSA.Alg()}))
	var claims refreshClaims
	_, err := parser.ParseWithClaims(tokenStr, &claims, func(t *jwt.Token) (interface{}, error) {
		return s.pub, nil
	})
	if err != nil {
		return nil, err
	}
	return &claims, nil
}

// VerifyOAuthCode verifies a JWT authorization code and returns the claims
func (s *Signer) VerifyOAuthCode(tokenStr string) (*oauthCodeClaims, error) {
	parser := jwt.NewParser(jwt.WithValidMethods([]string{jwt.SigningMethodEdDSA.Alg()}))
	var claims oauthCodeClaims
	_, err := parser.ParseWithClaims(tokenStr, &claims, func(t *jwt.Token) (interface{}, error) {
		return s.pub, nil
	})
	if err != nil {
		return nil, err
	}
	// Verify audience is for OAuth authorization code
	if !containsAny(claims.RegisteredClaims.Audience, []string{"oauth-authorization-code"}) {
		return nil, errors.New("invalid audience for OAuth code")
	}
	return &claims, nil
}


func containsAny(hay []string, needles []string) bool {
	for _, h := range hay {
		for _, n := range needles {
			if h == n {
				return true
			}
		}
	}
	return false
}

// JWKS returns a minimal Ed25519 JWKS for the current signer key
func (s *Signer) JWKS() map[string]any {
    x := base64.RawURLEncoding.EncodeToString(s.pub)
    key := map[string]any{
        "kty": "OKP",
        "crv": "Ed25519",
        "alg": "EdDSA",
        "use": "sig",
        "kid": s.kid,
        "x":   x,
    }
    return map[string]any{"keys": []any{key}}
}
