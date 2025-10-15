package s3compat

import (
    "bytes"
    "crypto/hmac"
    "crypto/sha256"
    "encoding/base64"
    "encoding/hex"
    "encoding/json"
    "encoding/xml"
    "errors"
    "fmt"
    "io"
    "net/http"
    "net/url"
    "os"
    "strings"
    "time"

    "github.com/aws/aws-sdk-go-v2/aws"
    v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
    authpkg "github.com/diggerhq/digger/opentaco/internal/auth"
    "github.com/diggerhq/digger/opentaco/internal/deps"
    "github.com/diggerhq/digger/opentaco/internal/domain"
    "github.com/diggerhq/digger/opentaco/internal/rbac"
    "github.com/diggerhq/digger/opentaco/internal/sts"
    "github.com/diggerhq/digger/opentaco/internal/storage"
    "github.com/labstack/echo/v4"
    "github.com/google/uuid"
)

// Handler implements minimal S3-compatible endpoint under /s3 with SigV4 verification.
// Supported keys: <bucket>/<unit-id>/terraform.tfstate(.lock|.tflock)
type Handler struct {
    store     domain.StateOperations 
    signer    *authpkg.Signer
    stsIssuer sts.Issuer
}

func NewHandler(store domain.StateOperations, signer *authpkg.Signer, stsIssuer sts.Issuer) *Handler {
    return &Handler{store: store, signer: signer, stsIssuer: stsIssuer}
}

// Handle routes GET/HEAD/PUT/DELETE for both tfstate and lock objects.
func (h *Handler) Handle(c echo.Context) error {
    // Handle bucket-level ListObjectsV2 (Terraform probes workspaces under env:/)
    if isListObjectsV2(c.Request()) {
        // Verify SigV4 first
        _, err := h.verifySigV4(c)
        if err != nil {
            var ae *authError
            if errors.As(err, &ae) && ae.code == http.StatusForbidden {
                return c.JSON(http.StatusForbidden, map[string]string{"error":"signature_mismatch"})
            }
            return c.JSON(http.StatusUnauthorized, map[string]string{"error":"unauthorized"})
        }
        // Note: RBAC checks are handled at the service level for S3-compatible operations
        return handleListObjectsV2(c)
    }

    // Parse object path
    obj, err := parsePath(c.Request().URL.Path)
    if err != nil {
        return c.NoContent(http.StatusNotFound)
    }

    // Verify SigV4 with OT stateless STS creds
    _, err = h.verifySigV4(c)
    if err != nil {
        // Normalize to 401 on auth errors; 403 on signature mismatch
        var ae *authError
        if errors.As(err, &ae) && ae.code == http.StatusForbidden {
            return c.JSON(http.StatusForbidden, map[string]string{"error":"signature_mismatch"})
        }
        return c.JSON(http.StatusUnauthorized, map[string]string{"error":"unauthorized"})
    }

    // Note: RBAC checks are handled at the service level for S3-compatible operations

    // Dispatch
    switch c.Request().Method {
    case http.MethodGet:
        if obj.isLock { return h.getLock(c, obj.unitID) }
        return h.getState(c, obj.unitID)
    case http.MethodHead:
        if obj.isLock { return h.headLock(c, obj.unitID) }
        return h.headState(c, obj.unitID)
    case http.MethodPut:
        if obj.isLock { return h.putLock(c, obj.unitID) }
        return h.putState(c, obj.unitID)
    case http.MethodDelete:
        if obj.isLock { return h.deleteLock(c, obj.unitID) }
        return c.NoContent(http.StatusMethodNotAllowed)
    default:
        return c.NoContent(http.StatusMethodNotAllowed)
    }
}

// isListObjectsV2 returns true for GET /s3/<bucket>?list-type=2
func isListObjectsV2(r *http.Request) bool {
    if r.Method != http.MethodGet { return false }
    p := strings.TrimPrefix(r.URL.Path, "/")
    if !strings.HasPrefix(p, "s3/") { return false }
    rest := strings.TrimPrefix(p, "s3/")
    // Allow /s3/<bucket> and /s3/<bucket>/ only
    if rest == "" { return false }
    if strings.Contains(rest, "/") {
        // If there is anything after the bucket segment (besides optional trailing slash), it's not a listing
        parts := strings.Split(rest, "/")
        if len(parts) > 1 && parts[1] != "" { return false }
    }
    return r.URL.Query().Get("list-type") == "2"
}

type listBucketResult struct {
    XMLName     xml.Name `xml:"ListBucketResult"`
    Xmlns       string   `xml:"xmlns,attr"`
    Name        string   `xml:"Name"`
    Prefix      string   `xml:"Prefix"`
    KeyCount    int      `xml:"KeyCount"`
    MaxKeys     int      `xml:"MaxKeys"`
    IsTruncated bool     `xml:"IsTruncated"`
}

func handleListObjectsV2(c echo.Context) error {
    // Minimal empty result sufficient for Terraform workspace probing
    // Path: /s3/<bucket>
    p := strings.TrimPrefix(c.Request().URL.Path, "/s3/")
    bucket := strings.Trim(p, "/")
    prefix := c.QueryParam("prefix")
    res := listBucketResult{
        Xmlns:       "http://s3.amazonaws.com/doc/2006-03-01/",
        Name:        bucket,
        Prefix:      prefix,
        KeyCount:    0,
        MaxKeys:     1000,
        IsTruncated: false,
    }
    c.Response().Header().Set("Content-Type", "application/xml")
    return xml.NewEncoder(c.Response()).Encode(res)
}

// --- Handlers ---

func (h *Handler) getState(c echo.Context, id string) error {
    // If state doesn't exist or is empty, return 404 to signal Terraform to initialize
    meta, err := h.store.Get(c.Request().Context(), id)
    if err != nil {
        if errors.Is(err, storage.ErrNotFound) { return c.NoContent(http.StatusNotFound) }
        return c.JSON(http.StatusInternalServerError, map[string]string{"error":"head_failed"})
    }
    if meta == nil || meta.Size == 0 {
        return c.NoContent(http.StatusNotFound)
    }
    data, err := h.store.Download(c.Request().Context(), id)
    if err != nil {
        if errors.Is(err, storage.ErrNotFound) { return c.NoContent(http.StatusNotFound) }
        return c.JSON(http.StatusInternalServerError, map[string]string{"error":"download_failed"})
    }
    if len(data) == 0 {
        return c.NoContent(http.StatusNotFound)
    }
    return c.Blob(http.StatusOK, "application/json", data)
}

func (h *Handler) headState(c echo.Context, id string) error {
    meta, err := h.store.Get(c.Request().Context(), id)
    if err != nil {
        if errors.Is(err, storage.ErrNotFound) { return c.NoContent(http.StatusNotFound) }
        return c.NoContent(http.StatusInternalServerError)
    }
    if meta == nil || meta.Size == 0 {
        return c.NoContent(http.StatusNotFound)
    }
    c.Response().Header().Set("Content-Length", fmt.Sprintf("%d", meta.Size))
    c.Response().Header().Set("Content-Type", "application/json")
    return c.NoContent(http.StatusOK)
}

func (h *Handler) putState(c echo.Context, id string) error {
    // Read body
    b, err := readAllAndReset(c.Request())
    if err != nil { return c.JSON(http.StatusBadRequest, map[string]string{"error":"read_failed"}) }

    // Check if state exists - error if not found (no auto-creation)
    if _, err := h.store.Get(c.Request().Context(), id); err == storage.ErrNotFound {
        return c.JSON(http.StatusNotFound, map[string]string{
            "error": "Unit not found. Please create the unit first using 'taco unit create " + id + "' or the opentaco_unit Terraform resource.",
        })
    } else if err != nil {
        return c.JSON(http.StatusInternalServerError, map[string]string{"error":"check_failed"})
    }

    // Optional lock ID (HTTP backend style). For S3 lockfile mode Terraform won't send it,
    // so if the state is locked, use the current lock's ID to satisfy storage semantics.
    lockID := c.Request().Header.Get("X-Terraform-Lock-ID")
    if lockID == "" { lockID = c.QueryParam("ID") }
    if lockID == "" { lockID = c.QueryParam("id") }
    if lockID == "" {
        if lk, _ := h.store.GetLock(c.Request().Context(), id); lk != nil {
            lockID = lk.ID
        }
    }

    if err := h.store.Upload(c.Request().Context(), id, b, lockID); err != nil {
        if errors.Is(err, storage.ErrLockConflict) {
            if lk, _ := h.store.GetLock(c.Request().Context(), id); lk != nil {
                return c.JSON(http.StatusConflict, lk)
            }
            return c.JSON(http.StatusConflict, map[string]string{"error":"locked"})
        }
        if errors.Is(err, storage.ErrNotFound) { return c.NoContent(http.StatusNotFound) }
        return c.JSON(http.StatusInternalServerError, map[string]string{"error":"upload_failed"})
    }
    // Best-effort dependency graph update
    go deps.UpdateGraphOnWrite(c.Request().Context(), h.store, id, b)
    return c.NoContent(http.StatusOK)
}

func (h *Handler) getLock(c echo.Context, id string) error {
    li, err := h.store.GetLock(c.Request().Context(), id)
    if err != nil { return c.JSON(http.StatusInternalServerError, map[string]string{"error":"lock_read_failed"}) }
    if li == nil { return c.NoContent(http.StatusNotFound) }
    return c.JSON(http.StatusOK, li)
}

func (h *Handler) headLock(c echo.Context, id string) error {
    li, err := h.store.GetLock(c.Request().Context(), id)
    if err != nil { return c.NoContent(http.StatusInternalServerError) }
    if li == nil { return c.NoContent(http.StatusNotFound) }
    // No body; set type for completeness
    c.Response().Header().Set("Content-Type", "application/json")
    return c.NoContent(http.StatusOK)
}

func (h *Handler) putLock(c echo.Context, id string) error {
    // Read request body; accept empty/invalid JSON by synthesizing a lock
    var li storage.LockInfo
    if err := json.NewDecoder(c.Request().Body).Decode(&li); err != nil {
        li = storage.LockInfo{ ID: uuid.New().String() }
    }
    if li.ID == "" { li.ID = uuid.New().String() }
    if li.Who == "" { li.Who = "terraform" }
    if li.Version == "" { li.Version = "1.0.0" }
    if li.Created.IsZero() { li.Created = time.Now() }

    // Attempt to lock (no auto-creation)
    if err := h.store.Lock(c.Request().Context(), id, &li); err != nil {
        if errors.Is(err, storage.ErrNotFound) {
            return c.JSON(http.StatusNotFound, map[string]string{
                "error": "Unit not found. Please create the unit first using 'taco unit create " + id + "' or the opentaco_unit Terraform resource.",
            })
        }
        if errors.Is(err, storage.ErrLockConflict) {
            if cur, _ := h.store.GetLock(c.Request().Context(), id); cur != nil {
                // Idempotent success if same lock ID
                if cur.ID == li.ID && li.ID != "" {
                    return c.JSON(http.StatusOK, cur)
                }
                return c.JSON(http.StatusLocked, cur)
            }
            return c.JSON(http.StatusConflict, map[string]string{"error":"already_locked"})
        }
        return c.JSON(http.StatusInternalServerError, map[string]string{"error":"lock_failed"})
    }
    return c.JSON(http.StatusOK, li)
}

func (h *Handler) deleteLock(c echo.Context, id string) error {
    // Determine lock ID
    var req struct{ ID string `json:"ID"` }
    _ = json.NewDecoder(c.Request().Body).Decode(&req)
    if req.ID == "" { req.ID = c.Request().Header.Get("X-Terraform-Lock-ID") }
    if req.ID == "" {
        // Fallback: use current lock's ID if present
        if cur, _ := h.store.GetLock(c.Request().Context(), id); cur != nil {
            req.ID = cur.ID
        }
    }
    if req.ID == "" { return c.JSON(http.StatusBadRequest, map[string]string{"error":"lock_id_required"}) }
    if err := h.store.Unlock(c.Request().Context(), id, req.ID); err != nil {
        if errors.Is(err, storage.ErrNotFound) { return c.NoContent(http.StatusNotFound) }
        if errors.Is(err, storage.ErrLockConflict) { return c.JSON(http.StatusConflict, map[string]string{"error":"lock_id_mismatch"}) }
        return c.JSON(http.StatusInternalServerError, map[string]string{"error":"unlock_failed"})
    }
    return c.NoContent(http.StatusOK)
}

// --- SigV4 verification ---

type authError struct{ code int; msg string }
func (e *authError) Error() string { return e.msg }

type parsedObject struct { unitID string; isLock bool }

func parsePath(path string) (*parsedObject, error) {
    // Expect /s3/<bucket>/<id>/terraform.tfstate[.lock|.tflock]
    p := strings.TrimPrefix(path, "/")
    if !strings.HasPrefix(p, "s3/") { return nil, errors.New("not s3 path") }
    p = strings.TrimPrefix(p, "s3/")
    parts := strings.Split(p, "/")
    if len(parts) < 2 { return nil, errors.New("invalid path") }
    keyParts := parts[1:]
    if len(keyParts) < 2 { return nil, errors.New("invalid key") }
    last := keyParts[len(keyParts)-1]
    isLock := (strings.HasSuffix(last, ".lock") || strings.HasSuffix(last, ".tflock")) && strings.HasPrefix(last, "terraform.tfstate")
    if last != "terraform.tfstate" && !isLock {
        return nil, errors.New("unknown object")
    }
    unitID := strings.Join(keyParts[:len(keyParts)-1], "/")
    return &parsedObject{unitID: unitID, isLock: isLock}, nil
}

func (h *Handler) verifySigV4(c echo.Context) (rbac.Principal, error) {
    req := c.Request()
    // Extract token (session token) required
    sessionTok := req.Header.Get("X-Amz-Security-Token")
    if sessionTok == "" { sessionTok = c.QueryParam("X-Amz-Security-Token") }
    if sessionTok == "" { return rbac.Principal{}, &authError{code: http.StatusUnauthorized, msg: "missing security token"} }

    if h.signer == nil { return rbac.Principal{}, &authError{code: http.StatusUnauthorized, msg: "signer unavailable"} }
    ac, err := h.signer.VerifyAccess(sessionTok)
    if err != nil { return rbac.Principal{}, &authError{code: http.StatusUnauthorized, msg: "invalid access token"} }
    // Require explicit s3 audience if provided
    audOK := false
    for _, a := range ac.RegisteredClaims.Audience { if a == "s3" { audOK = true; break } }
    if !audOK { return rbac.Principal{}, &authError{code: http.StatusUnauthorized, msg: "audience not allowed"} }

    // Parse credentials and scope
    sigHeader := req.Header.Get("Authorization")
    algo := ""
    credential := ""
    signatureProvided := ""
    // signedHeaders := ""
    var amzDate string
    if sigHeader != "" && strings.HasPrefix(sigHeader, "AWS4-HMAC-SHA256 ") {
        algo = "AWS4-HMAC-SHA256"
        // Parse params: Credential=..., SignedHeaders=..., Signature=...
        kvs := strings.Split(strings.TrimPrefix(sigHeader, "AWS4-HMAC-SHA256 "), ",")
        for _, kv := range kvs {
            kv = strings.TrimSpace(kv)
            if strings.HasPrefix(kv, "Credential=") { credential = strings.TrimPrefix(kv, "Credential=") }
            // if strings.HasPrefix(kv, "SignedHeaders=") { signedHeaders = strings.TrimPrefix(kv, "SignedHeaders=") }
            if strings.HasPrefix(kv, "Signature=") { signatureProvided = strings.TrimPrefix(kv, "Signature=") }
        }
        amzDate = req.Header.Get("X-Amz-Date")
    } else if q := req.URL.Query(); q.Get("X-Amz-Algorithm") == "AWS4-HMAC-SHA256" {
        algo = "AWS4-HMAC-SHA256"
        credential = q.Get("X-Amz-Credential")
        // signedHeaders = q.Get("X-Amz-SignedHeaders")
        signatureProvided = q.Get("X-Amz-Signature")
        amzDate = q.Get("X-Amz-Date")
    }
    if algo == "" || credential == "" || signatureProvided == "" || amzDate == "" {
        return rbac.Principal{}, &authError{code: http.StatusUnauthorized, msg: "missing signature"}
    }

    // Credential format: <AccessKeyID>/<Date>/<Region>/<Service>/aws4_request
    credParts := strings.Split(credential, "/")
    if len(credParts) < 5 { return rbac.Principal{}, &authError{code: http.StatusUnauthorized, msg: "invalid credential"} }
    accessKeyID := credParts[0]
    date := credParts[1]
    region := credParts[2]
    service := credParts[3]
    if service != "s3" { return rbac.Principal{}, &authError{code: http.StatusUnauthorized, msg: "invalid service"} }

    // Derive secret string from AccessKeyID: OTC.<kid>.<sid>
    secretStr, err := h.deriveSecretString(accessKeyID)
    if err != nil { return rbac.Principal{}, &authError{code: http.StatusUnauthorized, msg: "invalid access key"} }

    // Prepare signer inputs
    // Compute payload hash
    payloadHash := req.Header.Get("X-Amz-Content-Sha256")
    if payloadHash == "" || strings.EqualFold(payloadHash, "UNSIGNED-PAYLOAD") {
        // For GET/HEAD or when header says UNSIGNED-PAYLOAD
        payloadHash = "UNSIGNED-PAYLOAD"
    } else if payloadHash == "STREAMING-AWS4-HMAC-SHA256-PAYLOAD" {
        // Accept streaming payloads as unsigned for verification purposes here.
        payloadHash = "UNSIGNED-PAYLOAD"
    } else if req.Body != nil && (req.Method == http.MethodPut || req.Method == http.MethodPost) {
        // Re-hash body if header is inconsistent
        b, _ := readAllAndReset(req)
        sum := sha256.Sum256(b)
        payloadHash = hex.EncodeToString(sum[:])
    }

    // Parse signing time
    // X-Amz-Date: yyyymmddThhmmssZ, Scope date: yyyymmdd
    t, err := time.Parse("20060102T150405Z", amzDate)
    if err != nil {
        // try build from scope date if needed
        if len(date) == 8 {
            t, err = time.Parse("20060102", date)
            if err != nil { return rbac.Principal{}, &authError{code: http.StatusUnauthorized, msg: "bad date"} }
        } else {
            return rbac.Principal{}, &authError{code: http.StatusUnauthorized, msg: "bad date"}
        }
    }

    creds := aws.Credentials{AccessKeyID: accessKeyID, SecretAccessKey: secretStr, SessionToken: sessionTok, Source: "opentaco-sts"}
    signer := v4.NewSigner()

    // Work on a shallow clone of request to avoid mutating original headers for downstream logic
    cloned := req.Clone(req.Context())
    // Remove existing Authorization when present so signer can set a fresh one
    cloned.Header.Del("Authorization")

    if signatureProvided != "" && strings.Contains(strings.ToLower(req.URL.RawQuery), "x-amz-signature=") {
        // Presigned URL verification
        // Use provided query as-is; signer returns a new URL we can compare signature with
        // Determine expires if present
        // signer ignores mismatched expires in verification; we don't enforce it here.
        presignedURL, _, err := signer.PresignHTTP(c.Request().Context(), creds, cloned, payloadHash, service, region, t)
        if err != nil { return rbac.Principal{}, &authError{code: http.StatusForbidden, msg: "sign_error"} }
        u, _ := url.Parse(presignedURL)
        expSig := u.Query().Get("X-Amz-Signature")
        if expSig == "" || !secureCompare(expSig, signatureProvided) {
            return rbac.Principal{}, &authError{code: http.StatusForbidden, msg: "sig_mismatch"}
        }
    } else {
        // Header-based auth verification
        // Apply unsigned payload option when appropriate
        if err := signer.SignHTTP(c.Request().Context(), creds, cloned, payloadHash, service, region, t); err != nil {
            return rbac.Principal{}, &authError{code: http.StatusForbidden, msg: "sign_error"}
        }
        generated := cloned.Header.Get("Authorization")
        // Extract Signature= from generated header
        var expSig string
        if idx := strings.Index(generated, "Signature="); idx >= 0 {
            expSig = generated[idx+len("Signature="):]
            if i := strings.Index(expSig, ","); i >= 0 { expSig = expSig[:i] }
        }
        if expSig == "" || !secureCompare(expSig, signatureProvided) {
            return rbac.Principal{}, &authError{code: http.StatusForbidden, msg: "sig_mismatch"}
        }
    }

    // Build principal for RBAC
    princ := rbac.Principal{Subject: ac.Subject, Roles: ac.Roles, Groups: ac.Groups}
    return princ, nil
}

func (h *Handler) deriveSecretString(accessKeyID string) (string, error) {
    // OTC.<kid>.<sid>
    if !strings.HasPrefix(accessKeyID, "OTC.") { return "", errors.New("invalid akid") }
    parts := strings.Split(accessKeyID, ".")
    if len(parts) != 3 { return "", errors.New("invalid akid format") }
    kid := parts[1]
    sid := parts[2]
    // Prefer using in-memory issuer when possible
    if st, ok := h.stsIssuer.(*sts.StatelessIssuer); ok && st.KID() == kid {
        return st.DeriveSecretString(sid), nil
    }
    // Fallback: derive via env secret for the given kid
    env := os.Getenv("OPENTACO_STS_HMAC_" + kid)
    if env == "" { return "", errors.New("unknown kid") }
    b, err := base64.RawURLEncoding.DecodeString(env)
    if err != nil { return "", err }
    mac := hmac.New(sha256.New, b)
    mac.Write([]byte(sid))
    derived := mac.Sum(nil)
    return base64.RawURLEncoding.EncodeToString(derived), nil
}

// --- small helpers (local to avoid extra deps) ---

func secureCompare(a, b string) bool {
    if len(a) != len(b) { return false }
    var v byte
    for i := 0; i < len(a); i++ { v |= a[i] ^ b[i] }
    return v == 0
}

func readAllAndReset(r *http.Request) ([]byte, error) {
    var buf bytes.Buffer
    if r.Body == nil { return nil, nil }
    if _, err := buf.ReadFrom(r.Body); err != nil { return nil, err }
    r.Body.Close()
    r.Body = io.NopCloser(bytes.NewReader(buf.Bytes()))
    return buf.Bytes(), nil
}
