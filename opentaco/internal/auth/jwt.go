package auth

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
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
        priv, pub, err = loadEd25519FromPEM(pemPath)
        if err != nil {
            return nil, fmt.Errorf("load ed25519 pem: %w", err)
        }
    } else {
        // Dev: generate ephemeral
        pub, priv, err = ed25519.GenerateKey(nil)
        if err != nil {
            return nil, fmt.Errorf("generate ed25519: %w", err)
        }
    }

    kid := getenv("OPENTACO_TOKENS_KID", "k1")
    access := getenv("OPENTACO_TOKENS_ACCESS_TTL", "1h")
    refresh := getenv("OPENTACO_TOKENS_REFRESH_TTL", "720h")
    accessTTL, err := time.ParseDuration(access)
    if err != nil { accessTTL = time.Hour }
    refreshTTL, err := time.ParseDuration(refresh)
    if err != nil { refreshTTL = 30 * 24 * time.Hour }
    issuer := getenv("OPENTACO_PUBLIC_BASE_URL", "http://localhost:8080")

    return &Signer{priv: priv, pub: pub, kid: kid, accessTTL: accessTTL, refreshTTL: refreshTTL, issuer: issuer}, nil
}

func loadEd25519FromPEM(path string) (ed25519.PrivateKey, ed25519.PublicKey, error) {
    b, err := os.ReadFile(path)
    if err != nil { return nil, nil, err }
    block, _ := pem.Decode(b)
    if block == nil { return nil, nil, errors.New("invalid PEM") }
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
    Org    string   `json:"org,omitempty"`
    jwt.RegisteredClaims
}

type refreshClaims struct {
    RID string `json:"rid"`
    jwt.RegisteredClaims
}

func (s *Signer) MintAccess(sub string, roles, groups, scopes []string) (string, time.Time, error) {
    now := time.Now()
    exp := now.Add(s.accessTTL)
    claims := accessClaims{
        Roles:  roles,
        Groups: groups,
        Scopes: scopes,
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
    if err != nil { return nil, err }
    return &claims, nil
}

func (s *Signer) PublicJWK() map[string]any {
    // Ed25519 JWK fields: kty=OKP, crv=Ed25519, x=base64url(pub)
    return map[string]any{
        "kty": "OKP",
        "crv": "Ed25519",
        "kid": s.kid,
        "alg": "EdDSA",
        "use": "sig",
        "x":   b64RawURL(s.pub),
    }
}

func containsAny(hay []string, needles []string) bool {
    for _, h := range hay {
        for _, n := range needles {
            if h == n { return true }
        }
    }
    return false
}

func b64RawURL(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func getenv(k, d string) string {
    if v := os.Getenv(k); v != "" {
        return v
    }
    return d
}
