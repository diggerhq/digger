package storage

import (
    "bytes"
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "strings"
    "time"

    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/s3"
    "github.com/aws/smithy-go"
)

// s3Store implements StateStore backed by an S3 bucket
// Milestone 2+ — real storage using S3 (bucket-only). No external DB.
type s3Store struct {
    client *s3.Client
    bucket string
    prefix string
}

// NewS3Store creates a new S3-backed state store.
// Region can be empty to use the default AWS config chain.
func NewS3Store(ctx context.Context, bucket, prefix, region string) (StateStore, error) {
    if bucket == "" {
        return nil, fmt.Errorf("s3 bucket is required")
    }
    var (
        cfg aws.Config
        err error
    )
    if region != "" {
        cfg, err = config.LoadDefaultConfig(ctx, config.WithRegion(region))
    } else {
        cfg, err = config.LoadDefaultConfig(ctx)
    }
    if err != nil {
        return nil, err
    }
    cli := s3.NewFromConfig(cfg)
    return &s3Store{client: cli, bucket: bucket, prefix: strings.Trim(prefix, "/")}, nil
}

func (s *s3Store) key(parts ...string) string {
    // Join with prefix if provided; keep path-style IDs intact in keys
    key := strings.Join(parts, "/")
    if s.prefix != "" {
        return s.prefix + "/" + key
    }
    return key
}

// Object layout:
// <prefix>/<id>/terraform.tfstate
// <prefix>/<id>/terraform.tfstate.lock
func (s *s3Store) objKey(id string) string  { return s.key(strings.Trim(id, "/"), "terraform.tfstate") }
func (s *s3Store) lockKey(id string) string { return s.key(strings.Trim(id, "/"), "terraform.tfstate.lock") }

func isNotFound(err error) bool {
    if err == nil {
        return false
    }
    var apiErr smithy.APIError
    if errors.As(err, &apiErr) {
        // S3 commonly returns these codes for missing objects
        code := apiErr.ErrorCode()
        return code == "NotFound" || code == "NoSuchKey"
    }
    return false
}

// Note: use standard errors.As for error type assertions

// Create creates a new state entry with an empty state file
func (s *s3Store) Create(ctx context.Context, id string) (*StateMetadata, error) {
    // Check if state already exists
    _, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
        Bucket: &s.bucket,
        Key:    aws.String(s.objKey(id)),
    })
    if err == nil {
        return nil, ErrAlreadyExists
    }
    // For non-404 errors, return
    if !isNotFound(err) && err != nil {
        return nil, err
    }

    now := time.Now()
    meta := &StateMetadata{ID: id, Size: 0, Updated: now, Locked: false}

    // Write empty state
    if _, err := s.client.PutObject(ctx, &s3.PutObjectInput{
        Bucket: &s.bucket,
        Key:    aws.String(s.objKey(id)),
        Body:   bytes.NewReader(nil),
    }); err != nil {
        return nil, err
    }

    return meta, nil
}

func (s *s3Store) Get(ctx context.Context, id string) (*StateMetadata, error) {
    head, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
        Bucket: &s.bucket,
        Key:    aws.String(s.objKey(id)),
    })
    if err != nil {
        if isNotFound(err) {
            return nil, ErrNotFound
        }
        return nil, err
    }
    meta := StateMetadata{ID: id}
    if head.ContentLength != nil {
        meta.Size = *head.ContentLength
    }
    if head.LastModified != nil {
        meta.Updated = *head.LastModified
    } else {
        meta.Updated = time.Now()
    }
    // Enrich with lock info if present
    if li, _ := s.GetLock(ctx, id); li != nil {
        meta.Locked = true
        meta.LockInfo = li
    } else {
        meta.Locked = false
        meta.LockInfo = nil
    }
    return &meta, nil
}

func (s *s3Store) List(ctx context.Context, prefix string) ([]*StateMetadata, error) {
    // List terraform.tfstate objects under <prefix>/<id>/terraform.tfstate
    // Compute the list prefix correctly without introducing double slashes
    var listPrefix string
    if strings.TrimSpace(prefix) != "" {
        // When user passes a prefix, scope listing to that logical subtree
        listPrefix = s.key(strings.Trim(prefix, "/")) + "/"
    } else if s.prefix != "" {
        // Otherwise, limit to the store's base prefix if present
        listPrefix = s.prefix + "/"
    } else {
        listPrefix = ""
    }
    var token *string
    var outStates []*StateMetadata
    for {
        resp, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
            Bucket:            &s.bucket,
            Prefix:            aws.String(listPrefix),
            ContinuationToken: token,
        })
        if err != nil {
            return nil, err
        }
        for _, obj := range resp.Contents {
            k := aws.ToString(obj.Key)
            if !strings.HasSuffix(k, "/terraform.tfstate") {
                continue
            }
            // Derive ID from the object key
            trimmed := k
            if s.prefix != "" {
                trimmed = strings.TrimPrefix(trimmed, s.prefix+"/")
            }
            id := strings.TrimSuffix(trimmed, "/terraform.tfstate")
            // Use list metadata when available
            meta := &StateMetadata{ID: id, Size: aws.ToInt64(obj.Size)}
            if obj.LastModified != nil {
                meta.Updated = *obj.LastModified
            }
            // Lock info (optional; avoid another request during list)
            // To keep list lightweight, omit lock inspection here.
            meta.Locked = false
            meta.LockInfo = nil
            outStates = append(outStates, meta)
        }
        if aws.ToBool(resp.IsTruncated) && resp.NextContinuationToken != nil {
            token = resp.NextContinuationToken
            continue
        }
        break
    }
    return outStates, nil
}

func (s *s3Store) Delete(ctx context.Context, id string) error {
    // Check existence via state object
    _, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
        Bucket: &s.bucket,
        Key:    aws.String(s.objKey(id)),
    })
    if isNotFound(err) {
        return ErrNotFound
    }
    if err != nil {
        return err
    }
    // Best-effort deletes
    _, _ = s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: &s.bucket, Key: aws.String(s.objKey(id))})
    _, _ = s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: &s.bucket, Key: aws.String(s.lockKey(id))})
    return nil
}

func (s *s3Store) Download(ctx context.Context, id string) ([]byte, error) {
    out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
        Bucket: &s.bucket,
        Key:    aws.String(s.objKey(id)),
    })
    if err != nil {
        if isNotFound(err) {
            return nil, ErrNotFound
        }
        return nil, err
    }
    defer out.Body.Close()
    return io.ReadAll(out.Body)
}

func (s *s3Store) Upload(ctx context.Context, id string, data []byte, lockID string) error {
    meta, err := s.Get(ctx, id)
    if err != nil {
        return err
    }
    // Lock checks
    if lockID != "" && meta.LockInfo != nil && meta.LockInfo.ID != lockID {
        return ErrLockConflict
    }
    if lockID == "" && meta.Locked {
        return ErrLockConflict
    }
    // Upload state
    if _, err := s.client.PutObject(ctx, &s3.PutObjectInput{
        Bucket: &s.bucket,
        Key:    aws.String(s.objKey(id)),
        Body:   bytes.NewReader(data),
        ContentType: aws.String("application/json"),
    }); err != nil {
        return err
    }
    return nil
}

func (s *s3Store) Lock(ctx context.Context, id string, info *LockInfo) error {
    // Ensure state exists
    if _, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
        Bucket: &s.bucket,
        Key:    aws.String(s.objKey(id)),
    }); err != nil {
        if isNotFound(err) {
            return ErrNotFound
        }
        return err
    }
    // Check existing lock
    if _, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
        Bucket: &s.bucket,
        Key:    aws.String(s.lockKey(id)),
    }); err == nil {
        // Already locked
        return fmt.Errorf("%w: state already locked", ErrLockConflict)
    } else if !isNotFound(err) {
        return err
    }
    // Write lock info (no atomic create; acceptable for now)
    b, _ := json.Marshal(info)
    _, err := s.client.PutObject(ctx, &s3.PutObjectInput{
        Bucket: &s.bucket,
        Key:    aws.String(s.lockKey(id)),
        Body:   bytes.NewReader(b),
        ContentType: aws.String("application/json"),
    })
    return err
}

func (s *s3Store) Unlock(ctx context.Context, id string, lockID string) error {
    li, err := s.GetLock(ctx, id)
    if err != nil {
        if err == ErrNotFound {
            return ErrNotFound
        }
        return err
    }
    if li == nil {
        return fmt.Errorf("state is not locked")
    }
    if li.ID != lockID {
        return ErrLockConflict
    }
    _, err = s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
        Bucket: &s.bucket,
        Key:    aws.String(s.lockKey(id)),
    })
    return err
}

func (s *s3Store) GetLock(ctx context.Context, id string) (*LockInfo, error) {
    out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
        Bucket: &s.bucket,
        Key:    aws.String(s.lockKey(id)),
    })
    if err != nil {
        if isNotFound(err) {
            return nil, nil
        }
        return nil, err
    }
    defer out.Body.Close()
    b, err := io.ReadAll(out.Body)
    if err != nil {
        return nil, err
    }
    var li LockInfo
    if err := json.Unmarshal(b, &li); err != nil {
        return nil, err
    }
    return &li, nil
}
