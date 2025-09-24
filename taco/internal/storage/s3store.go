package storage

import (
    "bytes"
    "context"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "os"
    "path/filepath"
    "sort"
    "strconv"
    "strings"
    "time"

    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/s3"
    "github.com/aws/smithy-go"
    "github.com/google/uuid"
)

// s3Store implements UnitStore backed by an S3 bucket
// Milestone 2+ — real storage using S3 (bucket-only). No external DB.
type s3Store struct {
    client *s3.Client
    bucket string
    prefix string
}

// S3Store interface methods
func (s *s3Store) GetS3Client() *s3.Client {
    return s.client
}

func (s *s3Store) GetS3Bucket() string {
    return s.bucket
}

func (s *s3Store) GetS3Prefix() string {
    return s.prefix
}

func (s *s3Store) Key(parts ...string) string {
    return s.key(parts...)
}

// NewS3Store creates a new S3-backed unit store.
// Region can be empty to use the default AWS config chain.
func NewS3Store(ctx context.Context, bucket, prefix, region string) (UnitStore, error) {
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
// <prefix>/<id>/versions/v-20240115T143022.123456Z-a1b2c3d4.tfstate
func (s *s3Store) objKey(id string) string  { return s.key(strings.Trim(id, "/"), "terraform.tfstate") }
func (s *s3Store) lockKey(id string) string { return s.key(strings.Trim(id, "/"), "terraform.tfstate.lock") }

// versionKeyWithHash generates a version key with timestamp and content hash
func (s *s3Store) versionKeyWithHash(id string, timestamp time.Time, data []byte) string {
    // Use first 8 characters of SHA256 hash
    hash := sha256.Sum256(data)
    hashStr := hex.EncodeToString(hash[:4]) // First 4 bytes = 8 hex characters
    
    versionName := fmt.Sprintf("v-%s-%s.tfstate", 
        timestamp.UTC().Format("20060102T150405.000000Z"), 
        hashStr)
    return s.key(strings.Trim(id, "/"), "versions", versionName)
}

// versionKeyFromTimestamp generates a version key from timestamp and hash (for restore)
func (s *s3Store) versionKeyFromTimestamp(id string, timestamp time.Time, hash string) string {
    versionName := fmt.Sprintf("v-%s-%s.tfstate", 
        timestamp.UTC().Format("20060102T150405.000000Z"), 
        hash)
    return s.key(strings.Trim(id, "/"), "versions", versionName)
}

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

// Create creates a new unit entry with an empty state file
func (s *s3Store) Create(ctx context.Context, id string) (*UnitMetadata, error) {
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
    
    // Create proper initial Terraform state JSON
    initialState := `{
  "version": 4,
  "terraform_version": "1.0.0",
  "serial": 0,
  "lineage": "` + generateLineage() + `",
  "outputs": {},
  "resources": []
}`
    stateData := []byte(initialState)
    meta := &UnitMetadata{ID: id, Size: int64(len(stateData)), Updated: now, Locked: false}

    // Write initial state with proper JSON format
    if _, err := s.client.PutObject(ctx, &s3.PutObjectInput{
        Bucket: &s.bucket,
        Key:    aws.String(s.objKey(id)),
        Body:   bytes.NewReader(stateData),
        ContentType: aws.String("application/json"),
    }); err != nil {
        return nil, err
    }

    return meta, nil
}

func (s *s3Store) Get(ctx context.Context, id string) (*UnitMetadata, error) {
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
    meta := UnitMetadata{ID: id}
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
    
    // Try to load metadata (tags, description, org)
    enrichedMeta, err := s.loadMetadata(ctx, id)
    if err == nil {
        // Copy metadata fields
        meta.Tags = enrichedMeta.Tags
        meta.Description = enrichedMeta.Description
        meta.Organization = enrichedMeta.Organization
    }
    
    return &meta, nil
}

func (s *s3Store) List(ctx context.Context, prefix string) ([]*UnitMetadata, error) {
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
    var outStates []*UnitMetadata
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
            meta := &UnitMetadata{ID: id, Size: aws.ToInt64(obj.Size)}
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
    // Check existence via tfstate object
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

    // Archive current tfstate if it exists and has content
    if meta.Size > 0 {
        // Download current tfstate to get its content for hashing
        currentData, err := s.Download(ctx, id)
        if err != nil {
            return fmt.Errorf("failed to read current state for archiving: %w", err)
        }
        
        // Generate version key with hash of current content
        timestamp := time.Now().UTC()
        versionKey := s.versionKeyWithHash(id, timestamp, currentData)
        
        // Copy current to versioned location
        _, err = s.client.CopyObject(ctx, &s3.CopyObjectInput{
            Bucket:     &s.bucket,
            Key:        aws.String(versionKey),
            CopySource: aws.String(fmt.Sprintf("%s/%s", s.bucket, s.objKey(id))),
        })
        if err != nil {
            return fmt.Errorf("failed to archive current version: %w", err)
        }
        
        // Clean up old versions after successful archiving
        if err := s.cleanupOldVersions(ctx, id); err != nil {
            fmt.Printf("Warning: failed to cleanup old versions for %s: %v\n", id, err)
        }
    }

    // Upload new tfstate
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
    // Ensure tfstate object exists
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
        return fmt.Errorf("%w: unit already locked", ErrLockConflict)
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
        return fmt.Errorf("unit is not locked")
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

// ListVersions returns all versions for a given unit ID
func (s *s3Store) ListVersions(ctx context.Context, id string) ([]*VersionInfo, error) {
    versionsPrefix := s.key(strings.Trim(id, "/"), "versions") + "/"
    
    var token *string
    var versions []*VersionInfo
    for {
        resp, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
            Bucket:            &s.bucket,
            Prefix:            aws.String(versionsPrefix),
            ContinuationToken: token,
        })
        if err != nil {
            return nil, err
        }
        
        for _, obj := range resp.Contents {
            filename := filepath.Base(aws.ToString(obj.Key))
            
            // Parse: v-20240115T143022.123456Z-a1b2c3d4.tfstate
            if !strings.HasPrefix(filename, "v-") || !strings.HasSuffix(filename, ".tfstate") {
                continue
            }
            
            // Remove v- prefix and .tfstate suffix
            middle := strings.TrimPrefix(strings.TrimSuffix(filename, ".tfstate"), "v-")
            
            // Split on last dash to separate timestamp and hash
            lastDash := strings.LastIndex(middle, "-")
            if lastDash == -1 {
                continue // Skip malformed filenames
            }
            
            timestampStr := middle[:lastDash]
            hashStr := middle[lastDash+1:]
            
            timestamp, err := time.Parse("20060102T150405.000000Z", timestampStr)
            if err != nil {
                continue // Skip malformed timestamps
            }
            
            versions = append(versions, &VersionInfo{
                Timestamp: timestamp,
                Hash:      hashStr,
                Size:      aws.ToInt64(obj.Size),
                S3Key:     aws.ToString(obj.Key),
            })
        }
        
        if aws.ToBool(resp.IsTruncated) && resp.NextContinuationToken != nil {
            token = resp.NextContinuationToken
            continue
        }
        break
    }
    
    // Sort by timestamp (newest first)
    sort.Slice(versions, func(i, j int) bool {
        return versions[i].Timestamp.After(versions[j].Timestamp)
    })
    
    return versions, nil
}

// RestoreVersion restores a specific version to be the current unit tfstate
func (s *s3Store) RestoreVersion(ctx context.Context, id string, versionTimestamp time.Time, lockID string) error {
    // First, find the version with the matching timestamp
    versions, err := s.ListVersions(ctx, id)
    if err != nil {
        return fmt.Errorf("failed to list versions: %w", err)
    }
    
    var targetVersion *VersionInfo
    for _, version := range versions {
        if version.Timestamp.Equal(versionTimestamp) {
            targetVersion = version
            break
        }
    }
    
    if targetVersion == nil {
        return fmt.Errorf("version not found for timestamp: %s", versionTimestamp.Format("2006-01-02 15:04:05"))
    }
    
    // Perform lock checks
    meta, err := s.Get(ctx, id)
    if err != nil {
        return err
    }
    if lockID != "" && meta.LockInfo != nil && meta.LockInfo.ID != lockID {
        return ErrLockConflict
    }
    if lockID == "" && meta.Locked {
        return ErrLockConflict
    }

    // Archive current state if it exists and has content (same as Upload)
    if meta.Size > 0 {
        currentData, err := s.Download(ctx, id)
        if err != nil {
            return fmt.Errorf("failed to read current state for archiving: %w", err)
        }
        
        timestamp := time.Now().UTC()
        versionKey := s.versionKeyWithHash(id, timestamp, currentData)
        
        _, err = s.client.CopyObject(ctx, &s3.CopyObjectInput{
            Bucket:     &s.bucket,
            Key:        aws.String(versionKey),
            CopySource: aws.String(fmt.Sprintf("%s/%s", s.bucket, s.objKey(id))),
        })
        if err != nil {
            return fmt.Errorf("failed to archive current before restore: %w", err)
        }
    }

    // Copy the target version to current location
    _, err = s.client.CopyObject(ctx, &s3.CopyObjectInput{
        Bucket:     &s.bucket,
        Key:        aws.String(s.objKey(id)),
        CopySource: aws.String(fmt.Sprintf("%s/%s", s.bucket, targetVersion.S3Key)),
    })
    if err != nil {
        return fmt.Errorf("failed to restore version: %w", err)
    }

    return nil
}

// getMaxVersions returns the maximum number of versions to keep per state
// Defaults to 10 if OPENTACO_MAX_VERSIONS is not set or invalid
func (s *s3Store) getMaxVersions() int {
    if maxStr := os.Getenv("OPENTACO_MAX_VERSIONS"); maxStr != "" {
        if max, err := strconv.Atoi(maxStr); err == nil && max > 0 {
            return max
        }
    }
    return 10 // Default
}

// cleanupOldVersions removes old versions beyond the configured maximum
// Keeps the most recent versions and removes older ones
func (s *s3Store) cleanupOldVersions(ctx context.Context, id string) error {
    maxVersions := s.getMaxVersions()
    
    versions, err := s.ListVersions(ctx, id)
    if err != nil {
        return fmt.Errorf("failed to list versions: %w", err)
    }
    
    // Keep only the most recent N versions
    if len(versions) <= maxVersions {
        return nil // Nothing to clean up
    }
    
    // Delete oldest versions (versions are sorted newest first in ListVersions)
    versionsToDelete := versions[maxVersions:]
    var deleteErrors []string
    
    for _, version := range versionsToDelete {
        _, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
            Bucket: &s.bucket,
            Key:    aws.String(version.S3Key),
        })
        if err != nil {
            // Collect errors but continue with other deletions
            deleteErrors = append(deleteErrors, fmt.Sprintf("failed to delete %s: %v", version.S3Key, err))
        }
    }
    
    if len(deleteErrors) > 0 {
        return fmt.Errorf("cleanup partially failed: %s", strings.Join(deleteErrors, "; "))
    }
    
    return nil
}

// CreateWithMetadata creates a unit with tags and metadata
func (s *s3Store) CreateWithMetadata(ctx context.Context, id string, tags []string, description string, org string) (*UnitMetadata, error) {
    // First create the unit normally
    unitMeta, err := s.Create(ctx, id)
    if err != nil && err != ErrAlreadyExists {
        return nil, err
    }
    
    // Add metadata to the unit
    unitMeta.Tags = tags
    unitMeta.Description = description
    unitMeta.Organization = org
    
    // Store metadata as a separate S3 object
    err = s.storeMetadata(ctx, id, unitMeta)
    if err != nil {
        return nil, fmt.Errorf("failed to store metadata: %w", err)
    }
    
    return unitMeta, nil
}

// UpdateMetadata updates tags and metadata for an existing unit
func (s *s3Store) UpdateMetadata(ctx context.Context, id string, tags []string, description string, org string) error {
    // Get existing unit to verify it exists
    unitMeta, err := s.Get(ctx, id)
    if err != nil {
        return err
    }
    
    // Update metadata
    unitMeta.Tags = tags
    unitMeta.Description = description
    unitMeta.Organization = org
    unitMeta.Updated = time.Now()
    
    // Store updated metadata
    return s.storeMetadata(ctx, id, unitMeta)
}

// ListByTags queries units by tags within an organization
func (s *s3Store) ListByTags(ctx context.Context, org string, tags []string) ([]*UnitMetadata, error) {
    // Get all units first
    allUnits, err := s.List(ctx, "")
    if err != nil {
        return nil, err
    }
    
    var matchingUnits []*UnitMetadata
    
    for _, unit := range allUnits {
        // Load metadata for each unit
        enrichedUnit, err := s.loadMetadata(ctx, unit.ID)
        if err != nil {
            // Skip units without metadata
            continue
        }
        
        // Filter by organization
        if enrichedUnit.Organization != org {
            continue
        }
        
        // Check if unit has all required tags
        if s.hasAllTags(enrichedUnit.Tags, tags) {
            matchingUnits = append(matchingUnits, enrichedUnit)
        }
    }
    
    return matchingUnits, nil
}


// storeMetadata stores unit metadata in S3
func (s *s3Store) storeMetadata(ctx context.Context, id string, meta *UnitMetadata) error {
    metadataKey := s.metadataKey(id)
    
    metadataJSON, err := json.Marshal(meta)
    if err != nil {
        return err
    }
    
    _, err = s.client.PutObject(ctx, &s3.PutObjectInput{
        Bucket:      &s.bucket,
        Key:         &metadataKey,
        Body:        bytes.NewReader(metadataJSON),
        ContentType: aws.String("application/json"),
    })
    
    return err
}

// loadMetadata loads unit metadata from S3
func (s *s3Store) loadMetadata(ctx context.Context, id string) (*UnitMetadata, error) {
    metadataKey := s.metadataKey(id)
    
    result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
        Bucket: &s.bucket,
        Key:    &metadataKey,
    })
    if err != nil {
        if isNotFound(err) {
            return nil, ErrNotFound
        }
        return nil, err
    }
    defer result.Body.Close()
    
    metadataBytes, err := io.ReadAll(result.Body)
    if err != nil {
        return nil, err
    }
    
    var meta UnitMetadata
    if err := json.Unmarshal(metadataBytes, &meta); err != nil {
        return nil, err
    }
    
    return &meta, nil
}

// metadataKey generates the S3 key for unit metadata
func (s *s3Store) metadataKey(id string) string {
    return s.Key("metadata", id+".json")
}

// hasAllTags checks if a unit has all required tags
func (s *s3Store) hasAllTags(unitTags, requiredTags []string) bool {
    for _, required := range requiredTags {
        found := false
        for _, unitTag := range unitTags {
            if unitTag == required {
                found = true
                break
            }
        }
        if !found {
            return false
        }
    }
    return true
}

//
// Tag Management Implementation
//

// CreateTag creates a new tag with metadata
func (s *s3Store) CreateTag(ctx context.Context, name string, description string) (*TagMetadata, error) {
    // Check if tag already exists
    _, err := s.GetTag(ctx, name)
    if err == nil {
        return nil, ErrAlreadyExists
    }
    if err != ErrNotFound {
        return nil, err
    }
    
    now := time.Now()
    tag := &TagMetadata{
        Name:        name,
        Description: description,
        CreatedAt:   now,
        UpdatedAt:   now,
        UnitCount:   0,
    }
    
    // Store tag metadata
    tagKey := s.tagMetadataKey(name)
    tagJSON, err := json.Marshal(tag)
    if err != nil {
        return nil, err
    }
    
    _, err = s.client.PutObject(ctx, &s3.PutObjectInput{
        Bucket:      &s.bucket,
        Key:         &tagKey,
        Body:        bytes.NewReader(tagJSON),
        ContentType: aws.String("application/json"),
    })
    
    if err != nil {
        return nil, err
    }
    
    return tag, nil
}

// GetTag retrieves tag metadata
func (s *s3Store) GetTag(ctx context.Context, name string) (*TagMetadata, error) {
    tagKey := s.tagMetadataKey(name)
    
    result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
        Bucket: &s.bucket,
        Key:    &tagKey,
    })
    if err != nil {
        if isNotFound(err) {
            return nil, ErrNotFound
        }
        return nil, err
    }
    defer result.Body.Close()
    
    tagBytes, err := io.ReadAll(result.Body)
    if err != nil {
        return nil, err
    }
    
    var tag TagMetadata
    if err := json.Unmarshal(tagBytes, &tag); err != nil {
        return nil, err
    }
    
    // Update unit count dynamically
    count, err := s.getTagUnitCount(ctx, name)
    if err == nil {
        tag.UnitCount = count
    }
    
    return &tag, nil
}

// ListTags lists all available tags
func (s *s3Store) ListTags(ctx context.Context) ([]*TagMetadata, error) {
    prefix := s.Key("tags") + "/"
    
    resp, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
        Bucket: &s.bucket,
        Prefix: &prefix,
    })
    if err != nil {
        return nil, err
    }
    
    var tags []*TagMetadata
    for _, obj := range resp.Contents {
        if !strings.HasSuffix(*obj.Key, ".json") {
            continue
        }
        
        // Extract tag name from key
        tagName := strings.TrimPrefix(*obj.Key, prefix)
        tagName = strings.TrimSuffix(tagName, ".json")
        
        tag, err := s.GetTag(ctx, tagName)
        if err != nil {
            continue // Skip problematic tags
        }
        
        tags = append(tags, tag)
    }
    
    return tags, nil
}

// UpdateTag updates tag metadata
func (s *s3Store) UpdateTag(ctx context.Context, name string, description string) error {
    // Get existing tag
    tag, err := s.GetTag(ctx, name)
    if err != nil {
        return err
    }
    
    // Update description and timestamp
    tag.Description = description
    tag.UpdatedAt = time.Now()
    
    // Store updated tag
    tagKey := s.tagMetadataKey(name)
    tagJSON, err := json.Marshal(tag)
    if err != nil {
        return err
    }
    
    _, err = s.client.PutObject(ctx, &s3.PutObjectInput{
        Bucket:      &s.bucket,
        Key:         &tagKey,
        Body:        bytes.NewReader(tagJSON),
        ContentType: aws.String("application/json"),
    })
    
    return err
}

// DeleteTag deletes a tag and removes it from all units
func (s *s3Store) DeleteTag(ctx context.Context, name string) error {
    // First, remove the tag from all units that have it
    units, err := s.GetUnitsByTag(ctx, name)
    if err != nil && err != ErrNotFound {
        return fmt.Errorf("failed to get units with tag %s: %w", name, err)
    }
    
    // Remove tag from each unit
    for _, unit := range units {
        if err := s.RemoveTagFromUnit(ctx, unit.ID, name); err != nil {
            return fmt.Errorf("failed to remove tag %s from unit %s: %w", name, unit.ID, err)
        }
    }
    
    // Delete the tag metadata
    tagKey := s.tagMetadataKey(name)
    _, err = s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
        Bucket: &s.bucket,
        Key:    &tagKey,
    })
    
    return err
}

// AddTagToUnit adds a tag to a unit
func (s *s3Store) AddTagToUnit(ctx context.Context, unitID string, tagName string) error {
    // Verify tag exists
    _, err := s.GetTag(ctx, tagName)
    if err != nil {
        return fmt.Errorf("tag %s does not exist: %w", tagName, err)
    }
    
    // Get unit metadata
    unit, err := s.Get(ctx, unitID)
    if err != nil {
        return err
    }
    
    // Check if tag already exists on unit
    for _, existingTag := range unit.Tags {
        if existingTag == tagName {
            return nil // Already has the tag, no-op
        }
    }
    
    // Add tag to unit
    unit.Tags = append(unit.Tags, tagName)
    unit.Updated = time.Now()
    
    // Store updated metadata
    return s.storeMetadata(ctx, unitID, unit)
}

// RemoveTagFromUnit removes a tag from a unit
func (s *s3Store) RemoveTagFromUnit(ctx context.Context, unitID string, tagName string) error {
    // Get unit metadata
    unit, err := s.Get(ctx, unitID)
    if err != nil {
        return err
    }
    
    // Remove tag from unit's tags list
    var newTags []string
    found := false
    for _, existingTag := range unit.Tags {
        if existingTag != tagName {
            newTags = append(newTags, existingTag)
        } else {
            found = true
        }
    }
    
    if !found {
        return nil // Tag wasn't on the unit, no-op
    }
    
    // Update unit metadata
    unit.Tags = newTags
    unit.Updated = time.Now()
    
    // Store updated metadata
    return s.storeMetadata(ctx, unitID, unit)
}

// GetUnitsByTag returns all units that have a specific tag
func (s *s3Store) GetUnitsByTag(ctx context.Context, tagName string) ([]*UnitMetadata, error) {
    // Get all units and filter by tag
    allUnits, err := s.List(ctx, "")
    if err != nil {
        return nil, err
    }
    
    var matchingUnits []*UnitMetadata
    for _, unit := range allUnits {
        // Load metadata to get tags
        enrichedUnit, err := s.loadMetadata(ctx, unit.ID)
        if err != nil {
            continue // Skip units without metadata
        }
        
        // Copy basic unit info to enriched version
        enrichedUnit.Size = unit.Size
        enrichedUnit.Updated = unit.Updated
        enrichedUnit.Locked = unit.Locked
        enrichedUnit.LockInfo = unit.LockInfo
        
        // Check if unit has the tag
        for _, unitTag := range enrichedUnit.Tags {
            if unitTag == tagName {
                matchingUnits = append(matchingUnits, enrichedUnit)
                break
            }
        }
    }
    
    return matchingUnits, nil
}

// GetTagsForUnit returns all tags for a specific unit
func (s *s3Store) GetTagsForUnit(ctx context.Context, unitID string) ([]string, error) {
    unit, err := s.Get(ctx, unitID)
    if err != nil {
        return nil, err
    }
    
    return unit.Tags, nil
}

// Helper methods for tag management

// tagMetadataKey generates the S3 key for tag metadata
func (s *s3Store) tagMetadataKey(tagName string) string {
    return s.Key("tags", tagName+".json")
}

// getTagUnitCount counts how many units use a specific tag
func (s *s3Store) getTagUnitCount(ctx context.Context, tagName string) (int, error) {
    units, err := s.GetUnitsByTag(ctx, tagName)
    if err != nil {
        return 0, err
    }
    return len(units), nil
}

// generateLineage generates a unique UUID for Terraform state lineage
func generateLineage() string {
    return uuid.New().String()
}
