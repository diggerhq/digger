package rbac

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
    "github.com/aws/aws-sdk-go-v2/service/s3"
    "github.com/aws/smithy-go"
)

// s3RBACStore implements RBACStore backed by S3
type s3RBACStore struct {
    client *s3.Client
    bucket string
    prefix string
}

// NewS3RBACStore creates a new S3-backed RBAC store
func NewS3RBACStore(client *s3.Client, bucket, prefix string) RBACStore {
    return &s3RBACStore{
        client: client,
        bucket: bucket,
        prefix: strings.Trim(prefix, "/"),
    }
}

func (s *s3RBACStore) key(parts ...string) string {
    key := strings.Join(parts, "/")
    if s.prefix != "" {
        return s.prefix + "/" + key
    }
    return key
}

// RBAC object layout:
// <prefix>/rbac/config.json
// <prefix>/rbac/permissions/<permission-id>.json
// <prefix>/rbac/roles/<role-id>.json
// <prefix>/rbac/users/<subject>.json

func (s *s3RBACStore) configKey() string {
    return s.key("rbac", "config.json")
}

func (s *s3RBACStore) permissionKey(id string) string {
    return s.key("rbac", "permissions", id+".json")
}

func (s *s3RBACStore) roleKey(id string) string {
    return s.key("rbac", "roles", id+".json")
}

func (s *s3RBACStore) userKey(subject string) string {
    // Replace special characters in subject to make it S3-safe
    safeSubject := strings.ReplaceAll(subject, "/", "_")
    safeSubject = strings.ReplaceAll(safeSubject, ":", "_")
    return s.key("rbac", "users", safeSubject+".json")
}

func isNotFound(err error) bool {
    if err == nil {
        return false
    }
    var apiErr smithy.APIError
    if err != nil {
        if errors.As(err, &apiErr) {
            code := apiErr.ErrorCode()
            return code == "NotFound" || code == "NoSuchKey"
        }
    }
    return false
}

// Config management

func (s *s3RBACStore) GetConfig(ctx context.Context) (*RBACConfig, error) {
    out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
        Bucket: &s.bucket,
        Key:    aws.String(s.configKey()),
    })
    if err != nil {
        if isNotFound(err) {
            return nil, nil // Config doesn't exist yet
        }
        return nil, err
    }
    defer out.Body.Close()

    data, err := io.ReadAll(out.Body)
    if err != nil {
        return nil, err
    }

    var config RBACConfig
    if err := json.Unmarshal(data, &config); err != nil {
        return nil, err
    }

    return &config, nil
}

func (s *s3RBACStore) SetConfig(ctx context.Context, config *RBACConfig) error {
    data, err := json.Marshal(config)
    if err != nil {
        return err
    }

    _, err = s.client.PutObject(ctx, &s3.PutObjectInput{
        Bucket:      &s.bucket,
        Key:         aws.String(s.configKey()),
        Body:        bytes.NewReader(data),
        ContentType: aws.String("application/json"),
    })
    return err
}

// Permission management

func (s *s3RBACStore) CreatePermission(ctx context.Context, permission *Permission) error {
    data, err := json.Marshal(permission)
    if err != nil {
        return err
    }

    _, err = s.client.PutObject(ctx, &s3.PutObjectInput{
        Bucket:      &s.bucket,
        Key:         aws.String(s.permissionKey(permission.ID)),
        Body:        bytes.NewReader(data),
        ContentType: aws.String("application/json"),
    })
    return err
}

func (s *s3RBACStore) GetPermission(ctx context.Context, id string) (*Permission, error) {
    out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
        Bucket: &s.bucket,
        Key:    aws.String(s.permissionKey(id)),
    })
    if err != nil {
        if isNotFound(err) {
            return nil, fmt.Errorf("permission not found: %s", id)
        }
        return nil, err
    }
    defer out.Body.Close()

    data, err := io.ReadAll(out.Body)
    if err != nil {
        return nil, err
    }

    var permission Permission
    if err := json.Unmarshal(data, &permission); err != nil {
        return nil, err
    }

    return &permission, nil
}

func (s *s3RBACStore) ListPermissions(ctx context.Context) ([]*Permission, error) {
    permissionsPrefix := s.key("rbac", "permissions") + "/"
    
    var permissions []*Permission
    var token *string
    
    for {
        resp, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
            Bucket:            &s.bucket,
            Prefix:            aws.String(permissionsPrefix),
            ContinuationToken: token,
        })
        if err != nil {
            return nil, err
        }

        for _, obj := range resp.Contents {
            key := aws.ToString(obj.Key)
            if !strings.HasSuffix(key, ".json") {
                continue
            }

            // Extract permission ID from key
            permissionID := strings.TrimSuffix(strings.TrimPrefix(key, permissionsPrefix), ".json")
            
            permission, err := s.GetPermission(ctx, permissionID)
            if err != nil {
                continue // Skip invalid permissions
            }
            permissions = append(permissions, permission)
        }

        if aws.ToBool(resp.IsTruncated) && resp.NextContinuationToken != nil {
            token = resp.NextContinuationToken
            continue
        }
        break
    }

    return permissions, nil
}

func (s *s3RBACStore) DeletePermission(ctx context.Context, id string) error {
    _, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
        Bucket: &s.bucket,
        Key:    aws.String(s.permissionKey(id)),
    })
    return err
}

// Role management

func (s *s3RBACStore) CreateRole(ctx context.Context, role *Role) error {
    // For backward compatibility, set version to 1 if not set
    if role.Version == 0 {
        role.Version = 1
    }
    return s.saveRoleWithVersion(ctx, role)
}

func (s *s3RBACStore) saveRoleWithVersion(ctx context.Context, role *Role) error {
    data, err := json.Marshal(role)
    if err != nil {
        return err
    }

    // If this is an update (version > 1), check for version conflicts
    if role.Version > 1 {
        // Get current version to check for conflicts
        current, err := s.GetRole(ctx, role.ID)
        if err != nil {
            return err
        }
        if current != nil && current.Version != role.Version {
            return ErrVersionConflict
        }
    }

    // Increment version for the save
    role.Version++
    
    _, err = s.client.PutObject(ctx, &s3.PutObjectInput{
        Bucket:      &s.bucket,
        Key:         aws.String(s.roleKey(role.ID)),
        Body:        bytes.NewReader(data),
        ContentType: aws.String("application/json"),
    })
    
    // If save failed, decrement version back
    if err != nil {
        role.Version--
    }
    
    return err
}

func (s *s3RBACStore) GetRole(ctx context.Context, id string) (*Role, error) {
    out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
        Bucket: &s.bucket,
        Key:    aws.String(s.roleKey(id)),
    })
    if err != nil {
        if isNotFound(err) {
            return nil, fmt.Errorf("role not found: %s", id)
        }
        return nil, err
    }
    defer out.Body.Close()

    data, err := io.ReadAll(out.Body)
    if err != nil {
        return nil, err
    }

    var role Role
    if err := json.Unmarshal(data, &role); err != nil {
        return nil, err
    }

    return &role, nil
}

func (s *s3RBACStore) ListRoles(ctx context.Context) ([]*Role, error) {
    rolesPrefix := s.key("rbac", "roles") + "/"
    
    var roles []*Role
    var token *string
    
    for {
        resp, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
            Bucket:            &s.bucket,
            Prefix:            aws.String(rolesPrefix),
            ContinuationToken: token,
        })
        if err != nil {
            return nil, err
        }

        for _, obj := range resp.Contents {
            key := aws.ToString(obj.Key)
            if !strings.HasSuffix(key, ".json") {
                continue
            }

            // Extract role ID from key
            roleID := strings.TrimSuffix(strings.TrimPrefix(key, rolesPrefix), ".json")
            
            role, err := s.GetRole(ctx, roleID)
            if err != nil {
                continue // Skip invalid roles
            }
            roles = append(roles, role)
        }

        if aws.ToBool(resp.IsTruncated) && resp.NextContinuationToken != nil {
            token = resp.NextContinuationToken
            continue
        }
        break
    }

    return roles, nil
}

func (s *s3RBACStore) DeleteRole(ctx context.Context, id string) error {
    _, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
        Bucket: &s.bucket,
        Key:    aws.String(s.roleKey(id)),
    })
    return err
}

// User assignment management

func (s *s3RBACStore) AssignRole(ctx context.Context, subject, email, roleID string) error {
    const maxRetries = 3
    
    for attempt := 0; attempt < maxRetries; attempt++ {
        // Get existing assignment or create new one
        assignment, err := s.GetUserAssignment(ctx, subject)
        if err != nil && !strings.Contains(err.Error(), "not found") {
            return err
        }

        if assignment == nil {
            assignment = &UserAssignment{
                Subject:   subject,
                Email:     email,
                Roles:     []string{},
                CreatedAt: time.Now(),
                UpdatedAt: time.Now(),
                Version:   1,
            }
        } else {
            assignment.UpdatedAt = time.Now()
            if assignment.Email == "" {
                assignment.Email = email
            }
        }

        // Add role if not already assigned
        for _, existingRole := range assignment.Roles {
            if existingRole == roleID {
                return nil // Already assigned
            }
        }
        assignment.Roles = append(assignment.Roles, roleID)

        // Try to save with optimistic locking
        err = s.saveUserAssignmentWithVersion(ctx, assignment)
        if err == nil {
            return nil // Success
        }
        
        // If it's a version conflict and we have retries left, try again
        if strings.Contains(err.Error(), "version conflict") && attempt < maxRetries-1 {
            continue
        }
        
        return err
    }
    
    return fmt.Errorf("failed to assign role after %d attempts", maxRetries)
}

// AssignRoleByEmail assigns a role to a user by email (looks up subject)
func (s *s3RBACStore) AssignRoleByEmail(ctx context.Context, email, roleID string) error {
    // Find user by email
    assignment, err := s.GetUserAssignmentByEmail(ctx, email)
    if err != nil {
        return fmt.Errorf("user not found with email: %s", email)
    }
    
    return s.AssignRole(ctx, assignment.Subject, email, roleID)
}

func (s *s3RBACStore) RevokeRole(ctx context.Context, subject, roleID string) error {
    const maxRetries = 3
    
    for attempt := 0; attempt < maxRetries; attempt++ {
        assignment, err := s.GetUserAssignment(ctx, subject)
        if err != nil {
            return err
        }
        if assignment == nil {
            return fmt.Errorf("user assignment not found: %s", subject)
        }

        // Remove role from list
        var newRoles []string
        found := false
        for _, role := range assignment.Roles {
            if role != roleID {
                newRoles = append(newRoles, role)
            } else {
                found = true
            }
        }
        
        // If role wasn't found, nothing to do
        if !found {
            return nil
        }
        
        assignment.Roles = newRoles
        assignment.UpdatedAt = time.Now()

        // Try to save with optimistic locking
        err = s.saveUserAssignmentWithVersion(ctx, assignment)
        if err == nil {
            return nil // Success
        }
        
        // If it's a version conflict and we have retries left, try again
        if strings.Contains(err.Error(), "version conflict") && attempt < maxRetries-1 {
            continue
        }
        
        return err
    }
    
    return fmt.Errorf("failed to revoke role after %d attempts", maxRetries)
}

// RevokeRoleByEmail revokes a role from a user by email (looks up subject)
func (s *s3RBACStore) RevokeRoleByEmail(ctx context.Context, email, roleID string) error {
    // Find user by email
    assignment, err := s.GetUserAssignmentByEmail(ctx, email)
    if err != nil {
        return fmt.Errorf("user not found with email: %s", email)
    }
    
    return s.RevokeRole(ctx, assignment.Subject, roleID)
}

func (s *s3RBACStore) GetUserAssignment(ctx context.Context, subject string) (*UserAssignment, error) {
    out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
        Bucket: &s.bucket,
        Key:    aws.String(s.userKey(subject)),
    })
    if err != nil {
        if isNotFound(err) {
            return nil, fmt.Errorf("user assignment not found: %s", subject)
        }
        return nil, err
    }
    defer out.Body.Close()

    data, err := io.ReadAll(out.Body)
    if err != nil {
        return nil, err
    }

    var assignment UserAssignment
    if err := json.Unmarshal(data, &assignment); err != nil {
        return nil, err
    }

    return &assignment, nil
}

// GetUserAssignmentByEmail finds a user assignment by email address
func (s *s3RBACStore) GetUserAssignmentByEmail(ctx context.Context, email string) (*UserAssignment, error) {
    // List all user assignments and find by email
    assignments, err := s.ListUserAssignments(ctx)
    if err != nil {
        return nil, err
    }
    
    for _, assignment := range assignments {
        if assignment.Email == email {
            return assignment, nil
        }
    }
    
    return nil, fmt.Errorf("user assignment not found with email: %s", email)
}

func (s *s3RBACStore) ListUserAssignments(ctx context.Context) ([]*UserAssignment, error) {
    usersPrefix := s.key("rbac", "users") + "/"
    
    var assignments []*UserAssignment
    var token *string
    
    for {
        resp, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
            Bucket:            &s.bucket,
            Prefix:            aws.String(usersPrefix),
            ContinuationToken: token,
        })
        if err != nil {
            return nil, err
        }

        for _, obj := range resp.Contents {
            key := aws.ToString(obj.Key)
            if !strings.HasSuffix(key, ".json") {
                continue
            }

            // Extract subject from key
            subject := strings.TrimSuffix(strings.TrimPrefix(key, usersPrefix), ".json")
            // Convert back from safe format
            subject = strings.ReplaceAll(subject, "_", "/")
            subject = strings.ReplaceAll(subject, "_", ":")
            
            assignment, err := s.GetUserAssignment(ctx, subject)
            if err != nil {
                continue // Skip invalid assignments
            }
            assignments = append(assignments, assignment)
        }

        if aws.ToBool(resp.IsTruncated) && resp.NextContinuationToken != nil {
            token = resp.NextContinuationToken
            continue
        }
        break
    }

    return assignments, nil
}

func (s *s3RBACStore) saveUserAssignment(ctx context.Context, assignment *UserAssignment) error {
    // For backward compatibility, set version to 1 if not set
    if assignment.Version == 0 {
        assignment.Version = 1
    }
    return s.saveUserAssignmentWithVersion(ctx, assignment)
}

func (s *s3RBACStore) saveUserAssignmentWithVersion(ctx context.Context, assignment *UserAssignment) error {
    data, err := json.Marshal(assignment)
    if err != nil {
        return err
    }

    // If this is an update (version > 1), check for version conflicts
    if assignment.Version > 1 {
        // Get current version to check for conflicts
        current, err := s.GetUserAssignment(ctx, assignment.Subject)
        if err != nil {
            return err
        }
        if current != nil && current.Version != assignment.Version {
            return ErrVersionConflict
        }
    }

    // Increment version for the save
    assignment.Version++
    
    _, err = s.client.PutObject(ctx, &s3.PutObjectInput{
        Bucket:      &s.bucket,
        Key:         aws.String(s.userKey(assignment.Subject)),
        Body:        bytes.NewReader(data),
        ContentType: aws.String("application/json"),
    })
    
    // If save failed, decrement version back
    if err != nil {
        assignment.Version--
    }
    
    return err
}
