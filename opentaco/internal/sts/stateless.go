package sts

import (
    "crypto/hmac"
    "crypto/sha256"
    "encoding/base64"
    "errors"
    "fmt"
    "math/rand"
    "os"
    "time"
)

type StatelessIssuer struct {
    kid     string
    secret  []byte
    ttl     time.Duration
}

func NewStatelessIssuerFromEnv() (*StatelessIssuer, error) {
    kid := getenv("OPENTACO_STS_KID", "k1")
    secretEnv := os.Getenv("OPENTACO_STS_HMAC_" + kid)
    if secretEnv == "" {
        // Dev fallback: random secret (non-persistent)
        tmp := make([]byte, 32)
        rand.Read(tmp)
        secretEnv = base64.RawURLEncoding.EncodeToString(tmp)
    }
    sec, err := base64.RawURLEncoding.DecodeString(secretEnv)
    if err != nil {
        return nil, fmt.Errorf("invalid STS HMAC secret (base64url): %w", err)
    }
    ttlStr := getenv("OPENTACO_STS_TTL", "15m")
    ttl, err := time.ParseDuration(ttlStr)
    if err != nil { ttl = 15 * time.Minute }
    return &StatelessIssuer{kid: kid, secret: sec, ttl: ttl}, nil
}

func (s *StatelessIssuer) Issue(subject string, sessionToken string) (string, string, string, int64, error) {
    if subject == "" { return "", "", "", 0, errors.New("empty subject") }
    sid := randomID(12)
    accessKeyID := fmt.Sprintf("OTC.%s.%s", s.kid, sid)
    secret := deriveHMAC(s.secret, sid)
    // SessionToken is the OpenTaco access token; caller must supply it
    exp := time.Now().Add(s.ttl).UTC().Unix()
    return accessKeyID, base64.RawURLEncoding.EncodeToString(secret), sessionToken, exp, nil
}

func deriveHMAC(secret []byte, data string) []byte {
    h := hmac.New(sha256.New, secret)
    h.Write([]byte(data))
    return h.Sum(nil)
}

func randomID(n int) string {
    const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
    b := make([]byte, n)
    for i := range b {
        b[i] = alphabet[rand.Intn(len(alphabet))]
    }
    return string(b)
}

func getenv(k, d string) string {
    if v := os.Getenv(k); v != "" { return v }
    return d
}

