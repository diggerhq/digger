package models

import "database/sql"

type StorageBucketsSelect struct {
	AllowedMimeTypes  []sql.NullString `json:"allowed_mime_types"`
	AvifAutodetection sql.NullBool     `json:"avif_autodetection"`
	CreatedAt         sql.NullString   `json:"created_at"`
	FileSizeLimit     sql.NullInt64    `json:"file_size_limit"`
	Id                string           `json:"id"`
	Name              string           `json:"name"`
	Owner             sql.NullString   `json:"owner"`
	OwnerId           sql.NullString   `json:"owner_id"`
	Public            sql.NullBool     `json:"public"`
	UpdatedAt         sql.NullString   `json:"updated_at"`
}

type StorageBucketsInsert struct {
	AllowedMimeTypes  []sql.NullString `json:"allowed_mime_types"`
	AvifAutodetection sql.NullBool     `json:"avif_autodetection"`
	CreatedAt         sql.NullString   `json:"created_at"`
	FileSizeLimit     sql.NullInt64    `json:"file_size_limit"`
	Id                string           `json:"id"`
	Name              string           `json:"name"`
	Owner             sql.NullString   `json:"owner"`
	OwnerId           sql.NullString   `json:"owner_id"`
	Public            sql.NullBool     `json:"public"`
	UpdatedAt         sql.NullString   `json:"updated_at"`
}

type StorageBucketsUpdate struct {
	AllowedMimeTypes  []sql.NullString `json:"allowed_mime_types"`
	AvifAutodetection sql.NullBool     `json:"avif_autodetection"`
	CreatedAt         sql.NullString   `json:"created_at"`
	FileSizeLimit     sql.NullInt64    `json:"file_size_limit"`
	Id                sql.NullString   `json:"id"`
	Name              sql.NullString   `json:"name"`
	Owner             sql.NullString   `json:"owner"`
	OwnerId           sql.NullString   `json:"owner_id"`
	Public            sql.NullBool     `json:"public"`
	UpdatedAt         sql.NullString   `json:"updated_at"`
}

type StorageObjectsSelect struct {
	BucketId       sql.NullString   `json:"bucket_id"`
	CreatedAt      sql.NullString   `json:"created_at"`
	Id             string           `json:"id"`
	LastAccessedAt sql.NullString   `json:"last_accessed_at"`
	Metadata       interface{}      `json:"metadata"`
	Name           sql.NullString   `json:"name"`
	Owner          sql.NullString   `json:"owner"`
	OwnerId        sql.NullString   `json:"owner_id"`
	PathTokens     []sql.NullString `json:"path_tokens"`
	UpdatedAt      sql.NullString   `json:"updated_at"`
	Version        sql.NullString   `json:"version"`
}

type StorageObjectsInsert struct {
	BucketId       sql.NullString   `json:"bucket_id"`
	CreatedAt      sql.NullString   `json:"created_at"`
	Id             sql.NullString   `json:"id"`
	LastAccessedAt sql.NullString   `json:"last_accessed_at"`
	Metadata       interface{}      `json:"metadata"`
	Name           sql.NullString   `json:"name"`
	Owner          sql.NullString   `json:"owner"`
	OwnerId        sql.NullString   `json:"owner_id"`
	PathTokens     []sql.NullString `json:"path_tokens"`
	UpdatedAt      sql.NullString   `json:"updated_at"`
	Version        sql.NullString   `json:"version"`
}

type StorageObjectsUpdate struct {
	BucketId       sql.NullString   `json:"bucket_id"`
	CreatedAt      sql.NullString   `json:"created_at"`
	Id             sql.NullString   `json:"id"`
	LastAccessedAt sql.NullString   `json:"last_accessed_at"`
	Metadata       interface{}      `json:"metadata"`
	Name           sql.NullString   `json:"name"`
	Owner          sql.NullString   `json:"owner"`
	OwnerId        sql.NullString   `json:"owner_id"`
	PathTokens     []sql.NullString `json:"path_tokens"`
	UpdatedAt      sql.NullString   `json:"updated_at"`
	Version        sql.NullString   `json:"version"`
}

type StorageMigrationsSelect struct {
	ExecutedAt sql.NullString `json:"executed_at"`
	Hash       string         `json:"hash"`
	Id         int32          `json:"id"`
	Name       string         `json:"name"`
}

type StorageMigrationsInsert struct {
	ExecutedAt sql.NullString `json:"executed_at"`
	Hash       string         `json:"hash"`
	Id         int32          `json:"id"`
	Name       string         `json:"name"`
}

type StorageMigrationsUpdate struct {
	ExecutedAt sql.NullString `json:"executed_at"`
	Hash       sql.NullString `json:"hash"`
	Id         sql.NullInt32  `json:"id"`
	Name       sql.NullString `json:"name"`
}

type StorageS3MultipartUploadsSelect struct {
	BucketId        string         `json:"bucket_id"`
	CreatedAt       string         `json:"created_at"`
	Id              string         `json:"id"`
	InProgressSize  int64          `json:"in_progress_size"`
	Key             string         `json:"key"`
	OwnerId         sql.NullString `json:"owner_id"`
	UploadSignature string         `json:"upload_signature"`
	Version         string         `json:"version"`
}

type StorageS3MultipartUploadsInsert struct {
	BucketId        string         `json:"bucket_id"`
	CreatedAt       sql.NullString `json:"created_at"`
	Id              string         `json:"id"`
	InProgressSize  sql.NullInt64  `json:"in_progress_size"`
	Key             string         `json:"key"`
	OwnerId         sql.NullString `json:"owner_id"`
	UploadSignature string         `json:"upload_signature"`
	Version         string         `json:"version"`
}

type StorageS3MultipartUploadsUpdate struct {
	BucketId        sql.NullString `json:"bucket_id"`
	CreatedAt       sql.NullString `json:"created_at"`
	Id              sql.NullString `json:"id"`
	InProgressSize  sql.NullInt64  `json:"in_progress_size"`
	Key             sql.NullString `json:"key"`
	OwnerId         sql.NullString `json:"owner_id"`
	UploadSignature sql.NullString `json:"upload_signature"`
	Version         sql.NullString `json:"version"`
}

type StorageS3MultipartUploadsPartsSelect struct {
	BucketId   string         `json:"bucket_id"`
	CreatedAt  string         `json:"created_at"`
	Etag       string         `json:"etag"`
	Id         string         `json:"id"`
	Key        string         `json:"key"`
	OwnerId    sql.NullString `json:"owner_id"`
	PartNumber int32          `json:"part_number"`
	Size       int64          `json:"size"`
	UploadId   string         `json:"upload_id"`
	Version    string         `json:"version"`
}

type StorageS3MultipartUploadsPartsInsert struct {
	BucketId   string         `json:"bucket_id"`
	CreatedAt  sql.NullString `json:"created_at"`
	Etag       string         `json:"etag"`
	Id         sql.NullString `json:"id"`
	Key        string         `json:"key"`
	OwnerId    sql.NullString `json:"owner_id"`
	PartNumber int32          `json:"part_number"`
	Size       sql.NullInt64  `json:"size"`
	UploadId   string         `json:"upload_id"`
	Version    string         `json:"version"`
}

type StorageS3MultipartUploadsPartsUpdate struct {
	BucketId   sql.NullString `json:"bucket_id"`
	CreatedAt  sql.NullString `json:"created_at"`
	Etag       sql.NullString `json:"etag"`
	Id         sql.NullString `json:"id"`
	Key        sql.NullString `json:"key"`
	OwnerId    sql.NullString `json:"owner_id"`
	PartNumber sql.NullInt32  `json:"part_number"`
	Size       sql.NullInt64  `json:"size"`
	UploadId   sql.NullString `json:"upload_id"`
	Version    sql.NullString `json:"version"`
}

type PublicOrganizationMembersSelect struct {
	CreatedAt      string `json:"created_at"`
	Id             int64  `json:"id"`
	MemberId       string `json:"member_id"`
	MemberRole     string `json:"member_role"`
	OrganizationId string `json:"organization_id"`
}

type PublicOrganizationMembersInsert struct {
	CreatedAt      sql.NullString `json:"created_at"`
	Id             sql.NullInt64  `json:"id"`
	MemberId       string         `json:"member_id"`
	MemberRole     string         `json:"member_role"`
	OrganizationId string         `json:"organization_id"`
}

type PublicOrganizationMembersUpdate struct {
	CreatedAt      sql.NullString `json:"created_at"`
	Id             sql.NullInt64  `json:"id"`
	MemberId       sql.NullString `json:"member_id"`
	MemberRole     sql.NullString `json:"member_role"`
	OrganizationId sql.NullString `json:"organization_id"`
}

type PublicOrganizationsSelect struct {
	CreatedAt string `json:"created_at"`
	Id        string `json:"id"`
	Slug      string `json:"slug"`
	Title     string `json:"title"`
}

type PublicOrganizationsInsert struct {
	CreatedAt sql.NullString `json:"created_at"`
	Id        sql.NullString `json:"id"`
	Slug      sql.NullString `json:"slug"`
	Title     sql.NullString `json:"title"`
}

type PublicOrganizationsUpdate struct {
	CreatedAt sql.NullString `json:"created_at"`
	Id        sql.NullString `json:"id"`
	Slug      sql.NullString `json:"slug"`
	Title     sql.NullString `json:"title"`
}

type PublicUserProfilesSelect struct {
	AvatarUrl sql.NullString `json:"avatar_url"`
	CreatedAt string         `json:"created_at"`
	FullName  sql.NullString `json:"full_name"`
	Id        string         `json:"id"`
}

type PublicUserProfilesInsert struct {
	AvatarUrl sql.NullString `json:"avatar_url"`
	CreatedAt sql.NullString `json:"created_at"`
	FullName  sql.NullString `json:"full_name"`
	Id        string         `json:"id"`
}

type PublicUserProfilesUpdate struct {
	AvatarUrl sql.NullString `json:"avatar_url"`
	CreatedAt sql.NullString `json:"created_at"`
	FullName  sql.NullString `json:"full_name"`
	Id        sql.NullString `json:"id"`
}

type PublicCustomersSelect struct {
	OrganizationId   string `json:"organization_id"`
	StripeCustomerId string `json:"stripe_customer_id"`
}

type PublicCustomersInsert struct {
	OrganizationId   string `json:"organization_id"`
	StripeCustomerId string `json:"stripe_customer_id"`
}

type PublicCustomersUpdate struct {
	OrganizationId   sql.NullString `json:"organization_id"`
	StripeCustomerId sql.NullString `json:"stripe_customer_id"`
}

type PublicOrganizationJoinInvitationsSelect struct {
	CreatedAt               string         `json:"created_at"`
	Id                      string         `json:"id"`
	InviteeOrganizationRole string         `json:"invitee_organization_role"`
	InviteeUserEmail        string         `json:"invitee_user_email"`
	InviteeUserId           sql.NullString `json:"invitee_user_id"`
	InviterUserId           string         `json:"inviter_user_id"`
	OrganizationId          string         `json:"organization_id"`
	Status                  string         `json:"status"`
}

type PublicOrganizationJoinInvitationsInsert struct {
	CreatedAt               sql.NullString `json:"created_at"`
	Id                      sql.NullString `json:"id"`
	InviteeOrganizationRole sql.NullString `json:"invitee_organization_role"`
	InviteeUserEmail        string         `json:"invitee_user_email"`
	InviteeUserId           sql.NullString `json:"invitee_user_id"`
	InviterUserId           string         `json:"inviter_user_id"`
	OrganizationId          string         `json:"organization_id"`
	Status                  sql.NullString `json:"status"`
}

type PublicOrganizationJoinInvitationsUpdate struct {
	CreatedAt               sql.NullString `json:"created_at"`
	Id                      sql.NullString `json:"id"`
	InviteeOrganizationRole sql.NullString `json:"invitee_organization_role"`
	InviteeUserEmail        sql.NullString `json:"invitee_user_email"`
	InviteeUserId           sql.NullString `json:"invitee_user_id"`
	InviterUserId           sql.NullString `json:"inviter_user_id"`
	OrganizationId          sql.NullString `json:"organization_id"`
	Status                  sql.NullString `json:"status"`
}

type PublicOrganizationsPrivateInfoSelect struct {
	BillingAddress interface{} `json:"billing_address"`
	Id             string      `json:"id"`
	PaymentMethod  interface{} `json:"payment_method"`
}

type PublicOrganizationsPrivateInfoInsert struct {
	BillingAddress interface{} `json:"billing_address"`
	Id             string      `json:"id"`
	PaymentMethod  interface{} `json:"payment_method"`
}

type PublicOrganizationsPrivateInfoUpdate struct {
	BillingAddress interface{}    `json:"billing_address"`
	Id             sql.NullString `json:"id"`
	PaymentMethod  interface{}    `json:"payment_method"`
}

type PublicPricesSelect struct {
	Active          sql.NullBool   `json:"active"`
	Currency        sql.NullString `json:"currency"`
	Description     sql.NullString `json:"description"`
	Id              string         `json:"id"`
	Interval        sql.NullString `json:"interval"`
	IntervalCount   sql.NullInt64  `json:"interval_count"`
	Metadata        interface{}    `json:"metadata"`
	ProductId       sql.NullString `json:"product_id"`
	TrialPeriodDays sql.NullInt64  `json:"trial_period_days"`
	Type            sql.NullString `json:"type"`
	UnitAmount      sql.NullInt64  `json:"unit_amount"`
}

type PublicPricesInsert struct {
	Active          sql.NullBool   `json:"active"`
	Currency        sql.NullString `json:"currency"`
	Description     sql.NullString `json:"description"`
	Id              string         `json:"id"`
	Interval        sql.NullString `json:"interval"`
	IntervalCount   sql.NullInt64  `json:"interval_count"`
	Metadata        interface{}    `json:"metadata"`
	ProductId       sql.NullString `json:"product_id"`
	TrialPeriodDays sql.NullInt64  `json:"trial_period_days"`
	Type            sql.NullString `json:"type"`
	UnitAmount      sql.NullInt64  `json:"unit_amount"`
}

type PublicPricesUpdate struct {
	Active          sql.NullBool   `json:"active"`
	Currency        sql.NullString `json:"currency"`
	Description     sql.NullString `json:"description"`
	Id              sql.NullString `json:"id"`
	Interval        sql.NullString `json:"interval"`
	IntervalCount   sql.NullInt64  `json:"interval_count"`
	Metadata        interface{}    `json:"metadata"`
	ProductId       sql.NullString `json:"product_id"`
	TrialPeriodDays sql.NullInt64  `json:"trial_period_days"`
	Type            sql.NullString `json:"type"`
	UnitAmount      sql.NullInt64  `json:"unit_amount"`
}

type PublicProductsSelect struct {
	Active      sql.NullBool   `json:"active"`
	Description sql.NullString `json:"description"`
	Id          string         `json:"id"`
	Image       sql.NullString `json:"image"`
	Metadata    interface{}    `json:"metadata"`
	Name        sql.NullString `json:"name"`
}

type PublicProductsInsert struct {
	Active      sql.NullBool   `json:"active"`
	Description sql.NullString `json:"description"`
	Id          string         `json:"id"`
	Image       sql.NullString `json:"image"`
	Metadata    interface{}    `json:"metadata"`
	Name        sql.NullString `json:"name"`
}

type PublicProductsUpdate struct {
	Active      sql.NullBool   `json:"active"`
	Description sql.NullString `json:"description"`
	Id          sql.NullString `json:"id"`
	Image       sql.NullString `json:"image"`
	Metadata    interface{}    `json:"metadata"`
	Name        sql.NullString `json:"name"`
}

type PublicSubscriptionsSelect struct {
	CancelAt           sql.NullString `json:"cancel_at"`
	CancelAtPeriodEnd  sql.NullBool   `json:"cancel_at_period_end"`
	CanceledAt         sql.NullString `json:"canceled_at"`
	Created            string         `json:"created"`
	CurrentPeriodEnd   string         `json:"current_period_end"`
	CurrentPeriodStart string         `json:"current_period_start"`
	EndedAt            sql.NullString `json:"ended_at"`
	Id                 string         `json:"id"`
	Metadata           interface{}    `json:"metadata"`
	OrganizationId     sql.NullString `json:"organization_id"`
	PriceId            sql.NullString `json:"price_id"`
	Quantity           sql.NullInt64  `json:"quantity"`
	Status             sql.NullString `json:"status"`
	TrialEnd           sql.NullString `json:"trial_end"`
	TrialStart         sql.NullString `json:"trial_start"`
}

type PublicSubscriptionsInsert struct {
	CancelAt           sql.NullString `json:"cancel_at"`
	CancelAtPeriodEnd  sql.NullBool   `json:"cancel_at_period_end"`
	CanceledAt         sql.NullString `json:"canceled_at"`
	Created            string         `json:"created"`
	CurrentPeriodEnd   string         `json:"current_period_end"`
	CurrentPeriodStart string         `json:"current_period_start"`
	EndedAt            sql.NullString `json:"ended_at"`
	Id                 string         `json:"id"`
	Metadata           interface{}    `json:"metadata"`
	OrganizationId     sql.NullString `json:"organization_id"`
	PriceId            sql.NullString `json:"price_id"`
	Quantity           sql.NullInt64  `json:"quantity"`
	Status             sql.NullString `json:"status"`
	TrialEnd           sql.NullString `json:"trial_end"`
	TrialStart         sql.NullString `json:"trial_start"`
}

type PublicSubscriptionsUpdate struct {
	CancelAt           sql.NullString `json:"cancel_at"`
	CancelAtPeriodEnd  sql.NullBool   `json:"cancel_at_period_end"`
	CanceledAt         sql.NullString `json:"canceled_at"`
	Created            sql.NullString `json:"created"`
	CurrentPeriodEnd   sql.NullString `json:"current_period_end"`
	CurrentPeriodStart sql.NullString `json:"current_period_start"`
	EndedAt            sql.NullString `json:"ended_at"`
	Id                 sql.NullString `json:"id"`
	Metadata           interface{}    `json:"metadata"`
	OrganizationId     sql.NullString `json:"organization_id"`
	PriceId            sql.NullString `json:"price_id"`
	Quantity           sql.NullInt64  `json:"quantity"`
	Status             sql.NullString `json:"status"`
	TrialEnd           sql.NullString `json:"trial_end"`
	TrialStart         sql.NullString `json:"trial_start"`
}

type PublicUserPrivateInfoSelect struct {
	CreatedAt           sql.NullString `json:"created_at"`
	DefaultOrganization sql.NullString `json:"default_organization"`
	Id                  string         `json:"id"`
}

type PublicUserPrivateInfoInsert struct {
	CreatedAt           sql.NullString `json:"created_at"`
	DefaultOrganization sql.NullString `json:"default_organization"`
	Id                  string         `json:"id"`
}

type PublicUserPrivateInfoUpdate struct {
	CreatedAt           sql.NullString `json:"created_at"`
	DefaultOrganization sql.NullString `json:"default_organization"`
	Id                  sql.NullString `json:"id"`
}

type PublicOrganizationCreditsSelect struct {
	Credits        int64  `json:"credits"`
	OrganizationId string `json:"organization_id"`
}

type PublicOrganizationCreditsInsert struct {
	Credits        sql.NullInt64 `json:"credits"`
	OrganizationId string        `json:"organization_id"`
}

type PublicOrganizationCreditsUpdate struct {
	Credits        sql.NullInt64  `json:"credits"`
	OrganizationId sql.NullString `json:"organization_id"`
}

type PublicProjectsSelect struct {
	ConfigurationYaml   sql.NullString   `json:"configuration_yaml"`
	CreatedAt           string           `json:"created_at"`
	DeletedAt           sql.NullString   `json:"deleted_at"`
	Id                  string           `json:"id"`
	IsGenerated         sql.NullBool     `json:"is_generated"`
	IsInMainBranch      sql.NullBool     `json:"is_in_main_branch"`
	IsManagingState     sql.NullBool     `json:"is_managing_state"`
	Labels              []sql.NullString `json:"labels"`
	LatestActionOn      sql.NullString   `json:"latest_action_on"`
	Name                string           `json:"name"`
	OrganizationId      string           `json:"organization_id"`
	ProjectStatus       string           `json:"project_status"`
	RepoId              int64            `json:"repo_id"`
	Slug                string           `json:"slug"`
	Status              sql.NullString   `json:"status"`
	TeamId              sql.NullInt64    `json:"team_id"`
	TerraformWorkingDir sql.NullString   `json:"terraform_working_dir"`
	UpdatedAt           string           `json:"updated_at"`
}

type PublicProjectsInsert struct {
	ConfigurationYaml   sql.NullString   `json:"configuration_yaml"`
	CreatedAt           sql.NullString   `json:"created_at"`
	DeletedAt           sql.NullString   `json:"deleted_at"`
	Id                  sql.NullString   `json:"id"`
	IsGenerated         sql.NullBool     `json:"is_generated"`
	IsInMainBranch      sql.NullBool     `json:"is_in_main_branch"`
	IsManagingState     sql.NullBool     `json:"is_managing_state"`
	Labels              []sql.NullString `json:"labels"`
	LatestActionOn      sql.NullString   `json:"latest_action_on"`
	Name                string           `json:"name"`
	OrganizationId      string           `json:"organization_id"`
	ProjectStatus       sql.NullString   `json:"project_status"`
	RepoId              sql.NullInt64    `json:"repo_id"`
	Slug                sql.NullString   `json:"slug"`
	Status              sql.NullString   `json:"status"`
	TeamId              sql.NullInt64    `json:"team_id"`
	TerraformWorkingDir sql.NullString   `json:"terraform_working_dir"`
	UpdatedAt           sql.NullString   `json:"updated_at"`
}

type PublicProjectsUpdate struct {
	ConfigurationYaml   sql.NullString   `json:"configuration_yaml"`
	CreatedAt           sql.NullString   `json:"created_at"`
	DeletedAt           sql.NullString   `json:"deleted_at"`
	Id                  sql.NullString   `json:"id"`
	IsGenerated         sql.NullBool     `json:"is_generated"`
	IsInMainBranch      sql.NullBool     `json:"is_in_main_branch"`
	IsManagingState     sql.NullBool     `json:"is_managing_state"`
	Labels              []sql.NullString `json:"labels"`
	LatestActionOn      sql.NullString   `json:"latest_action_on"`
	Name                sql.NullString   `json:"name"`
	OrganizationId      sql.NullString   `json:"organization_id"`
	ProjectStatus       sql.NullString   `json:"project_status"`
	RepoId              sql.NullInt64    `json:"repo_id"`
	Slug                sql.NullString   `json:"slug"`
	Status              sql.NullString   `json:"status"`
	TeamId              sql.NullInt64    `json:"team_id"`
	TerraformWorkingDir sql.NullString   `json:"terraform_working_dir"`
	UpdatedAt           sql.NullString   `json:"updated_at"`
}

type PublicProjectCommentsSelect struct {
	CreatedAt sql.NullString `json:"created_at"`
	Id        int64          `json:"id"`
	InReplyTo sql.NullInt64  `json:"in_reply_to"`
	ProjectId string         `json:"project_id"`
	Text      string         `json:"text"`
	UserId    string         `json:"user_id"`
}

type PublicProjectCommentsInsert struct {
	CreatedAt sql.NullString `json:"created_at"`
	Id        sql.NullInt64  `json:"id"`
	InReplyTo sql.NullInt64  `json:"in_reply_to"`
	ProjectId string         `json:"project_id"`
	Text      string         `json:"text"`
	UserId    string         `json:"user_id"`
}

type PublicProjectCommentsUpdate struct {
	CreatedAt sql.NullString `json:"created_at"`
	Id        sql.NullInt64  `json:"id"`
	InReplyTo sql.NullInt64  `json:"in_reply_to"`
	ProjectId sql.NullString `json:"project_id"`
	Text      sql.NullString `json:"text"`
	UserId    sql.NullString `json:"user_id"`
}

type PublicInternalBlogPostsSelect struct {
	Content     string         `json:"content"`
	CoverImage  sql.NullString `json:"cover_image"`
	CreatedAt   string         `json:"created_at"`
	Id          string         `json:"id"`
	IsFeatured  bool           `json:"is_featured"`
	JsonContent interface{}    `json:"json_content"`
	SeoData     interface{}    `json:"seo_data"`
	Slug        string         `json:"slug"`
	Status      string         `json:"status"`
	Summary     string         `json:"summary"`
	Title       string         `json:"title"`
	UpdatedAt   string         `json:"updated_at"`
}

type PublicInternalBlogPostsInsert struct {
	Content     string         `json:"content"`
	CoverImage  sql.NullString `json:"cover_image"`
	CreatedAt   sql.NullString `json:"created_at"`
	Id          sql.NullString `json:"id"`
	IsFeatured  sql.NullBool   `json:"is_featured"`
	JsonContent interface{}    `json:"json_content"`
	SeoData     interface{}    `json:"seo_data"`
	Slug        string         `json:"slug"`
	Status      sql.NullString `json:"status"`
	Summary     string         `json:"summary"`
	Title       string         `json:"title"`
	UpdatedAt   sql.NullString `json:"updated_at"`
}

type PublicInternalBlogPostsUpdate struct {
	Content     sql.NullString `json:"content"`
	CoverImage  sql.NullString `json:"cover_image"`
	CreatedAt   sql.NullString `json:"created_at"`
	Id          sql.NullString `json:"id"`
	IsFeatured  sql.NullBool   `json:"is_featured"`
	JsonContent interface{}    `json:"json_content"`
	SeoData     interface{}    `json:"seo_data"`
	Slug        sql.NullString `json:"slug"`
	Status      sql.NullString `json:"status"`
	Summary     sql.NullString `json:"summary"`
	Title       sql.NullString `json:"title"`
	UpdatedAt   sql.NullString `json:"updated_at"`
}

type PublicInternalBlogAuthorProfilesSelect struct {
	AvatarUrl       string         `json:"avatar_url"`
	Bio             string         `json:"bio"`
	CreatedAt       string         `json:"created_at"`
	DisplayName     string         `json:"display_name"`
	FacebookHandle  sql.NullString `json:"facebook_handle"`
	InstagramHandle sql.NullString `json:"instagram_handle"`
	LinkedinHandle  sql.NullString `json:"linkedin_handle"`
	TwitterHandle   sql.NullString `json:"twitter_handle"`
	UpdatedAt       string         `json:"updated_at"`
	UserId          string         `json:"user_id"`
	WebsiteUrl      sql.NullString `json:"website_url"`
}

type PublicInternalBlogAuthorProfilesInsert struct {
	AvatarUrl       string         `json:"avatar_url"`
	Bio             string         `json:"bio"`
	CreatedAt       sql.NullString `json:"created_at"`
	DisplayName     string         `json:"display_name"`
	FacebookHandle  sql.NullString `json:"facebook_handle"`
	InstagramHandle sql.NullString `json:"instagram_handle"`
	LinkedinHandle  sql.NullString `json:"linkedin_handle"`
	TwitterHandle   sql.NullString `json:"twitter_handle"`
	UpdatedAt       sql.NullString `json:"updated_at"`
	UserId          string         `json:"user_id"`
	WebsiteUrl      sql.NullString `json:"website_url"`
}

type PublicInternalBlogAuthorProfilesUpdate struct {
	AvatarUrl       sql.NullString `json:"avatar_url"`
	Bio             sql.NullString `json:"bio"`
	CreatedAt       sql.NullString `json:"created_at"`
	DisplayName     sql.NullString `json:"display_name"`
	FacebookHandle  sql.NullString `json:"facebook_handle"`
	InstagramHandle sql.NullString `json:"instagram_handle"`
	LinkedinHandle  sql.NullString `json:"linkedin_handle"`
	TwitterHandle   sql.NullString `json:"twitter_handle"`
	UpdatedAt       sql.NullString `json:"updated_at"`
	UserId          sql.NullString `json:"user_id"`
	WebsiteUrl      sql.NullString `json:"website_url"`
}

type PublicInternalBlogAuthorPostsSelect struct {
	AuthorId string `json:"author_id"`
	PostId   string `json:"post_id"`
}

type PublicInternalBlogAuthorPostsInsert struct {
	AuthorId string `json:"author_id"`
	PostId   string `json:"post_id"`
}

type PublicInternalBlogAuthorPostsUpdate struct {
	AuthorId sql.NullString `json:"author_id"`
	PostId   sql.NullString `json:"post_id"`
}

type PublicInternalBlogPostTagsSelect struct {
	Description sql.NullString `json:"description"`
	Id          int32          `json:"id"`
	Name        string         `json:"name"`
	Slug        string         `json:"slug"`
}

type PublicInternalBlogPostTagsInsert struct {
	Description sql.NullString `json:"description"`
	Id          sql.NullInt32  `json:"id"`
	Name        string         `json:"name"`
	Slug        string         `json:"slug"`
}

type PublicInternalBlogPostTagsUpdate struct {
	Description sql.NullString `json:"description"`
	Id          sql.NullInt32  `json:"id"`
	Name        sql.NullString `json:"name"`
	Slug        sql.NullString `json:"slug"`
}

type PublicInternalBlogPostTagsRelationshipSelect struct {
	BlogPostId string `json:"blog_post_id"`
	TagId      int32  `json:"tag_id"`
}

type PublicInternalBlogPostTagsRelationshipInsert struct {
	BlogPostId string `json:"blog_post_id"`
	TagId      int32  `json:"tag_id"`
}

type PublicInternalBlogPostTagsRelationshipUpdate struct {
	BlogPostId sql.NullString `json:"blog_post_id"`
	TagId      sql.NullInt32  `json:"tag_id"`
}

type PublicInternalChangelogSelect struct {
	Changes    string         `json:"changes"`
	CoverImage sql.NullString `json:"cover_image"`
	CreatedAt  sql.NullString `json:"created_at"`
	Id         string         `json:"id"`
	Title      string         `json:"title"`
	UpdatedAt  sql.NullString `json:"updated_at"`
	UserId     sql.NullString `json:"user_id"`
}

type PublicInternalChangelogInsert struct {
	Changes    string         `json:"changes"`
	CoverImage sql.NullString `json:"cover_image"`
	CreatedAt  sql.NullString `json:"created_at"`
	Id         sql.NullString `json:"id"`
	Title      string         `json:"title"`
	UpdatedAt  sql.NullString `json:"updated_at"`
	UserId     sql.NullString `json:"user_id"`
}

type PublicInternalChangelogUpdate struct {
	Changes    sql.NullString `json:"changes"`
	CoverImage sql.NullString `json:"cover_image"`
	CreatedAt  sql.NullString `json:"created_at"`
	Id         sql.NullString `json:"id"`
	Title      sql.NullString `json:"title"`
	UpdatedAt  sql.NullString `json:"updated_at"`
	UserId     sql.NullString `json:"user_id"`
}

type PublicInternalFeedbackThreadsSelect struct {
	AddedToRoadmap          bool   `json:"added_to_roadmap"`
	Content                 string `json:"content"`
	CreatedAt               string `json:"created_at"`
	Id                      string `json:"id"`
	IsPubliclyVisible       bool   `json:"is_publicly_visible"`
	OpenForPublicDiscussion bool   `json:"open_for_public_discussion"`
	Priority                string `json:"priority"`
	Status                  string `json:"status"`
	Title                   string `json:"title"`
	Type                    string `json:"type"`
	UpdatedAt               string `json:"updated_at"`
	UserId                  string `json:"user_id"`
}

type PublicInternalFeedbackThreadsInsert struct {
	AddedToRoadmap          sql.NullBool   `json:"added_to_roadmap"`
	Content                 string         `json:"content"`
	CreatedAt               sql.NullString `json:"created_at"`
	Id                      sql.NullString `json:"id"`
	IsPubliclyVisible       sql.NullBool   `json:"is_publicly_visible"`
	OpenForPublicDiscussion sql.NullBool   `json:"open_for_public_discussion"`
	Priority                sql.NullString `json:"priority"`
	Status                  sql.NullString `json:"status"`
	Title                   string         `json:"title"`
	Type                    sql.NullString `json:"type"`
	UpdatedAt               sql.NullString `json:"updated_at"`
	UserId                  string         `json:"user_id"`
}

type PublicInternalFeedbackThreadsUpdate struct {
	AddedToRoadmap          sql.NullBool   `json:"added_to_roadmap"`
	Content                 sql.NullString `json:"content"`
	CreatedAt               sql.NullString `json:"created_at"`
	Id                      sql.NullString `json:"id"`
	IsPubliclyVisible       sql.NullBool   `json:"is_publicly_visible"`
	OpenForPublicDiscussion sql.NullBool   `json:"open_for_public_discussion"`
	Priority                sql.NullString `json:"priority"`
	Status                  sql.NullString `json:"status"`
	Title                   sql.NullString `json:"title"`
	Type                    sql.NullString `json:"type"`
	UpdatedAt               sql.NullString `json:"updated_at"`
	UserId                  sql.NullString `json:"user_id"`
}

type PublicInternalFeedbackCommentsSelect struct {
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
	Id        string `json:"id"`
	ThreadId  string `json:"thread_id"`
	UpdatedAt string `json:"updated_at"`
	UserId    string `json:"user_id"`
}

type PublicInternalFeedbackCommentsInsert struct {
	Content   string         `json:"content"`
	CreatedAt sql.NullString `json:"created_at"`
	Id        sql.NullString `json:"id"`
	ThreadId  string         `json:"thread_id"`
	UpdatedAt sql.NullString `json:"updated_at"`
	UserId    string         `json:"user_id"`
}

type PublicInternalFeedbackCommentsUpdate struct {
	Content   sql.NullString `json:"content"`
	CreatedAt sql.NullString `json:"created_at"`
	Id        sql.NullString `json:"id"`
	ThreadId  sql.NullString `json:"thread_id"`
	UpdatedAt sql.NullString `json:"updated_at"`
	UserId    sql.NullString `json:"user_id"`
}

type PublicUserNotificationsSelect struct {
	CreatedAt string         `json:"created_at"`
	Id        string         `json:"id"`
	IsRead    bool           `json:"is_read"`
	IsSeen    bool           `json:"is_seen"`
	Payload   interface{}    `json:"payload"`
	UpdatedAt string         `json:"updated_at"`
	UserId    sql.NullString `json:"user_id"`
}

type PublicUserNotificationsInsert struct {
	CreatedAt sql.NullString `json:"created_at"`
	Id        sql.NullString `json:"id"`
	IsRead    sql.NullBool   `json:"is_read"`
	IsSeen    sql.NullBool   `json:"is_seen"`
	Payload   interface{}    `json:"payload"`
	UpdatedAt sql.NullString `json:"updated_at"`
	UserId    sql.NullString `json:"user_id"`
}

type PublicUserNotificationsUpdate struct {
	CreatedAt sql.NullString `json:"created_at"`
	Id        sql.NullString `json:"id"`
	IsRead    sql.NullBool   `json:"is_read"`
	IsSeen    sql.NullBool   `json:"is_seen"`
	Payload   interface{}    `json:"payload"`
	UpdatedAt sql.NullString `json:"updated_at"`
	UserId    sql.NullString `json:"user_id"`
}

type PublicUserApiKeysSelect struct {
	CreatedAt string         `json:"created_at"`
	ExpiresAt sql.NullString `json:"expires_at"`
	IsRevoked bool           `json:"is_revoked"`
	KeyId     string         `json:"key_id"`
	MaskedKey string         `json:"masked_key"`
	UserId    string         `json:"user_id"`
}

type PublicUserApiKeysInsert struct {
	CreatedAt sql.NullString `json:"created_at"`
	ExpiresAt sql.NullString `json:"expires_at"`
	IsRevoked sql.NullBool   `json:"is_revoked"`
	KeyId     string         `json:"key_id"`
	MaskedKey string         `json:"masked_key"`
	UserId    string         `json:"user_id"`
}

type PublicUserApiKeysUpdate struct {
	CreatedAt sql.NullString `json:"created_at"`
	ExpiresAt sql.NullString `json:"expires_at"`
	IsRevoked sql.NullBool   `json:"is_revoked"`
	KeyId     sql.NullString `json:"key_id"`
	MaskedKey sql.NullString `json:"masked_key"`
	UserId    sql.NullString `json:"user_id"`
}

type PublicUserRolesSelect struct {
	Id     int64  `json:"id"`
	Role   string `json:"role"`
	UserId string `json:"user_id"`
}

type PublicUserRolesInsert struct {
	Id     sql.NullInt64 `json:"id"`
	Role   string        `json:"role"`
	UserId string        `json:"user_id"`
}

type PublicUserRolesUpdate struct {
	Id     sql.NullInt64  `json:"id"`
	Role   sql.NullString `json:"role"`
	UserId sql.NullString `json:"user_id"`
}

type PublicUserOnboardingSelect struct {
	AcceptedTerms bool   `json:"accepted_terms"`
	CreatedAt     string `json:"created_at"`
	UserId        string `json:"user_id"`
}

type PublicUserOnboardingInsert struct {
	AcceptedTerms sql.NullBool   `json:"accepted_terms"`
	CreatedAt     sql.NullString `json:"created_at"`
	UserId        string         `json:"user_id"`
}

type PublicUserOnboardingUpdate struct {
	AcceptedTerms sql.NullBool   `json:"accepted_terms"`
	CreatedAt     sql.NullString `json:"created_at"`
	UserId        sql.NullString `json:"user_id"`
}

type PublicAccountDeleteTokensSelect struct {
	Token  string `json:"token"`
	UserId string `json:"user_id"`
}

type PublicAccountDeleteTokensInsert struct {
	Token  sql.NullString `json:"token"`
	UserId string         `json:"user_id"`
}

type PublicAccountDeleteTokensUpdate struct {
	Token  sql.NullString `json:"token"`
	UserId sql.NullString `json:"user_id"`
}

type PublicChatsSelect struct {
	CreatedAt string         `json:"created_at"`
	Id        string         `json:"id"`
	Payload   interface{}    `json:"payload"`
	ProjectId string         `json:"project_id"`
	UserId    sql.NullString `json:"user_id"`
}

type PublicChatsInsert struct {
	CreatedAt sql.NullString `json:"created_at"`
	Id        string         `json:"id"`
	Payload   interface{}    `json:"payload"`
	ProjectId string         `json:"project_id"`
	UserId    sql.NullString `json:"user_id"`
}

type PublicChatsUpdate struct {
	CreatedAt sql.NullString `json:"created_at"`
	Id        sql.NullString `json:"id"`
	Payload   interface{}    `json:"payload"`
	ProjectId sql.NullString `json:"project_id"`
	UserId    sql.NullString `json:"user_id"`
}

type PublicReposSelect struct {
	CreatedAt      sql.NullString `json:"created_at"`
	DeletedAt      sql.NullString `json:"deleted_at"`
	DiggerConfig   sql.NullString `json:"digger_config"`
	Id             int64          `json:"id"`
	Name           string         `json:"name"`
	OrganizationId sql.NullString `json:"organization_id"`
	UpdatedAt      sql.NullString `json:"updated_at"`
}

type PublicReposInsert struct {
	CreatedAt      sql.NullString `json:"created_at"`
	DeletedAt      sql.NullString `json:"deleted_at"`
	DiggerConfig   sql.NullString `json:"digger_config"`
	Id             sql.NullInt64  `json:"id"`
	Name           string         `json:"name"`
	OrganizationId sql.NullString `json:"organization_id"`
	UpdatedAt      sql.NullString `json:"updated_at"`
}

type PublicReposUpdate struct {
	CreatedAt      sql.NullString `json:"created_at"`
	DeletedAt      sql.NullString `json:"deleted_at"`
	DiggerConfig   sql.NullString `json:"digger_config"`
	Id             sql.NullInt64  `json:"id"`
	Name           sql.NullString `json:"name"`
	OrganizationId sql.NullString `json:"organization_id"`
	UpdatedAt      sql.NullString `json:"updated_at"`
}

type PublicDiggerBatchesSelect struct {
	BatchType            string         `json:"batch_type"`
	BranchName           string         `json:"branch_name"`
	CommentId            sql.NullInt64  `json:"comment_id"`
	DiggerConfig         sql.NullString `json:"digger_config"`
	GithubInstallationId sql.NullInt64  `json:"github_installation_id"`
	GitlabProjectId      sql.NullInt64  `json:"gitlab_project_id"`
	Id                   string         `json:"id"`
	PrNumber             sql.NullInt64  `json:"pr_number"`
	RepoFullName         string         `json:"repo_full_name"`
	RepoName             string         `json:"repo_name"`
	RepoOwner            string         `json:"repo_owner"`
	SourceDetails        []byte         `json:"source_details"`
	Status               int16          `json:"status"`
	Vcs                  sql.NullString `json:"vcs"`
}

type PublicDiggerBatchesInsert struct {
	BatchType            string         `json:"batch_type"`
	BranchName           string         `json:"branch_name"`
	CommentId            sql.NullInt64  `json:"comment_id"`
	DiggerConfig         sql.NullString `json:"digger_config"`
	GithubInstallationId sql.NullInt64  `json:"github_installation_id"`
	GitlabProjectId      sql.NullInt64  `json:"gitlab_project_id"`
	Id                   sql.NullString `json:"id"`
	PrNumber             sql.NullInt64  `json:"pr_number"`
	RepoFullName         string         `json:"repo_full_name"`
	RepoName             string         `json:"repo_name"`
	RepoOwner            string         `json:"repo_owner"`
	SourceDetails        []byte         `json:"source_details"`
	Status               int16          `json:"status"`
	Vcs                  sql.NullString `json:"vcs"`
}

type PublicDiggerBatchesUpdate struct {
	BatchType            sql.NullString `json:"batch_type"`
	BranchName           sql.NullString `json:"branch_name"`
	CommentId            sql.NullInt64  `json:"comment_id"`
	DiggerConfig         sql.NullString `json:"digger_config"`
	GithubInstallationId sql.NullInt64  `json:"github_installation_id"`
	GitlabProjectId      sql.NullInt64  `json:"gitlab_project_id"`
	Id                   sql.NullString `json:"id"`
	PrNumber             sql.NullInt64  `json:"pr_number"`
	RepoFullName         sql.NullString `json:"repo_full_name"`
	RepoName             sql.NullString `json:"repo_name"`
	RepoOwner            sql.NullString `json:"repo_owner"`
	SourceDetails        []byte         `json:"source_details"`
	Status               sql.NullInt32  `json:"status"`
	Vcs                  sql.NullString `json:"vcs"`
}

type PublicDiggerJobSummariesSelect struct {
	CreatedAt        string         `json:"created_at"`
	DeletedAt        sql.NullString `json:"deleted_at"`
	Id               string         `json:"id"`
	ResourcesCreated int64          `json:"resources_created"`
	ResourcesDeleted int64          `json:"resources_deleted"`
	ResourcesUpdated int64          `json:"resources_updated"`
	UpdatedAt        string         `json:"updated_at"`
}

type PublicDiggerJobSummariesInsert struct {
	CreatedAt        sql.NullString `json:"created_at"`
	DeletedAt        sql.NullString `json:"deleted_at"`
	Id               sql.NullString `json:"id"`
	ResourcesCreated sql.NullInt64  `json:"resources_created"`
	ResourcesDeleted sql.NullInt64  `json:"resources_deleted"`
	ResourcesUpdated sql.NullInt64  `json:"resources_updated"`
	UpdatedAt        sql.NullString `json:"updated_at"`
}

type PublicDiggerJobSummariesUpdate struct {
	CreatedAt        sql.NullString `json:"created_at"`
	DeletedAt        sql.NullString `json:"deleted_at"`
	Id               sql.NullString `json:"id"`
	ResourcesCreated sql.NullInt64  `json:"resources_created"`
	ResourcesDeleted sql.NullInt64  `json:"resources_deleted"`
	ResourcesUpdated sql.NullInt64  `json:"resources_updated"`
	UpdatedAt        sql.NullString `json:"updated_at"`
}

type PublicDiggerJobsSelect struct {
	BatchId            string         `json:"batch_id"`
	CreatedAt          string         `json:"created_at"`
	DeletedAt          sql.NullString `json:"deleted_at"`
	DiggerJobId        string         `json:"digger_job_id"`
	DiggerJobSummaryId sql.NullString `json:"digger_job_summary_id"`
	Id                 string         `json:"id"`
	PlanFootprint      []byte         `json:"plan_footprint"`
	PrCommentUrl       sql.NullString `json:"pr_comment_url"`
	Status             int16          `json:"status"`
	StatusUpdatedAt    sql.NullString `json:"status_updated_at"`
	TerraformOutput    sql.NullString `json:"terraform_output"`
	UpdatedAt          string         `json:"updated_at"`
	WorkflowFile       sql.NullString `json:"workflow_file"`
	WorkflowRunUrl     sql.NullString `json:"workflow_run_url"`
}

type PublicDiggerJobsInsert struct {
	BatchId            string         `json:"batch_id"`
	CreatedAt          sql.NullString `json:"created_at"`
	DeletedAt          sql.NullString `json:"deleted_at"`
	DiggerJobId        string         `json:"digger_job_id"`
	DiggerJobSummaryId sql.NullString `json:"digger_job_summary_id"`
	Id                 sql.NullString `json:"id"`
	PlanFootprint      []byte         `json:"plan_footprint"`
	PrCommentUrl       sql.NullString `json:"pr_comment_url"`
	Status             int16          `json:"status"`
	StatusUpdatedAt    sql.NullString `json:"status_updated_at"`
	TerraformOutput    sql.NullString `json:"terraform_output"`
	UpdatedAt          sql.NullString `json:"updated_at"`
	WorkflowFile       sql.NullString `json:"workflow_file"`
	WorkflowRunUrl     sql.NullString `json:"workflow_run_url"`
}

type PublicDiggerJobsUpdate struct {
	BatchId            sql.NullString `json:"batch_id"`
	CreatedAt          sql.NullString `json:"created_at"`
	DeletedAt          sql.NullString `json:"deleted_at"`
	DiggerJobId        sql.NullString `json:"digger_job_id"`
	DiggerJobSummaryId sql.NullString `json:"digger_job_summary_id"`
	Id                 sql.NullString `json:"id"`
	PlanFootprint      []byte         `json:"plan_footprint"`
	PrCommentUrl       sql.NullString `json:"pr_comment_url"`
	Status             sql.NullInt32  `json:"status"`
	StatusUpdatedAt    sql.NullString `json:"status_updated_at"`
	TerraformOutput    sql.NullString `json:"terraform_output"`
	UpdatedAt          sql.NullString `json:"updated_at"`
	WorkflowFile       sql.NullString `json:"workflow_file"`
	WorkflowRunUrl     sql.NullString `json:"workflow_run_url"`
}

type PublicDiggerLocksSelect struct {
	CreatedAt      string         `json:"created_at"`
	DeletedAt      sql.NullString `json:"deleted_at"`
	Id             string         `json:"id"`
	LockId         int64          `json:"lock_id"`
	OrganizationId string         `json:"organization_id"`
	Resource       string         `json:"resource"`
	UpdatedAt      string         `json:"updated_at"`
}

type PublicDiggerLocksInsert struct {
	CreatedAt      sql.NullString `json:"created_at"`
	DeletedAt      sql.NullString `json:"deleted_at"`
	Id             sql.NullString `json:"id"`
	LockId         int64          `json:"lock_id"`
	OrganizationId string         `json:"organization_id"`
	Resource       string         `json:"resource"`
	UpdatedAt      sql.NullString `json:"updated_at"`
}

type PublicDiggerLocksUpdate struct {
	CreatedAt      sql.NullString `json:"created_at"`
	DeletedAt      sql.NullString `json:"deleted_at"`
	Id             sql.NullString `json:"id"`
	LockId         sql.NullInt64  `json:"lock_id"`
	OrganizationId sql.NullString `json:"organization_id"`
	Resource       sql.NullString `json:"resource"`
	UpdatedAt      sql.NullString `json:"updated_at"`
}

type PublicDiggerRunStagesSelect struct {
	BatchId   string         `json:"batch_id"`
	CreatedAt string         `json:"created_at"`
	DeletedAt sql.NullString `json:"deleted_at"`
	Id        string         `json:"id"`
	UpdatedAt string         `json:"updated_at"`
}

type PublicDiggerRunStagesInsert struct {
	BatchId   string         `json:"batch_id"`
	CreatedAt sql.NullString `json:"created_at"`
	DeletedAt sql.NullString `json:"deleted_at"`
	Id        sql.NullString `json:"id"`
	UpdatedAt sql.NullString `json:"updated_at"`
}

type PublicDiggerRunStagesUpdate struct {
	BatchId   sql.NullString `json:"batch_id"`
	CreatedAt sql.NullString `json:"created_at"`
	DeletedAt sql.NullString `json:"deleted_at"`
	Id        sql.NullString `json:"id"`
	UpdatedAt sql.NullString `json:"updated_at"`
}

type PublicDiggerRunsSelect struct {
	ApplyStageId         sql.NullString `json:"apply_stage_id"`
	ApprovalAuthor       sql.NullString `json:"approval_author"`
	ApprovalDate         sql.NullString `json:"approval_date"`
	CommitId             string         `json:"commit_id"`
	CreatedAt            string         `json:"created_at"`
	DeletedAt            sql.NullString `json:"deleted_at"`
	DiggerConfig         sql.NullString `json:"digger_config"`
	GithubInstallationId sql.NullInt64  `json:"github_installation_id"`
	Id                   string         `json:"id"`
	IsApproved           sql.NullBool   `json:"is_approved"`
	PlanStageId          sql.NullString `json:"plan_stage_id"`
	PrNumber             sql.NullInt64  `json:"pr_number"`
	ProjectName          sql.NullString `json:"project_name"`
	RepoId               int64          `json:"repo_id"`
	RunType              string         `json:"run_type"`
	Status               string         `json:"status"`
	Triggertype          string         `json:"triggertype"`
	UpdatedAt            string         `json:"updated_at"`
}

type PublicDiggerRunsInsert struct {
	ApplyStageId         sql.NullString `json:"apply_stage_id"`
	ApprovalAuthor       sql.NullString `json:"approval_author"`
	ApprovalDate         sql.NullString `json:"approval_date"`
	CommitId             string         `json:"commit_id"`
	CreatedAt            sql.NullString `json:"created_at"`
	DeletedAt            sql.NullString `json:"deleted_at"`
	DiggerConfig         sql.NullString `json:"digger_config"`
	GithubInstallationId sql.NullInt64  `json:"github_installation_id"`
	Id                   sql.NullString `json:"id"`
	IsApproved           sql.NullBool   `json:"is_approved"`
	PlanStageId          sql.NullString `json:"plan_stage_id"`
	PrNumber             sql.NullInt64  `json:"pr_number"`
	ProjectName          sql.NullString `json:"project_name"`
	RepoId               sql.NullInt64  `json:"repo_id"`
	RunType              string         `json:"run_type"`
	Status               string         `json:"status"`
	Triggertype          string         `json:"triggertype"`
	UpdatedAt            sql.NullString `json:"updated_at"`
}

type PublicDiggerRunsUpdate struct {
	ApplyStageId         sql.NullString `json:"apply_stage_id"`
	ApprovalAuthor       sql.NullString `json:"approval_author"`
	ApprovalDate         sql.NullString `json:"approval_date"`
	CommitId             sql.NullString `json:"commit_id"`
	CreatedAt            sql.NullString `json:"created_at"`
	DeletedAt            sql.NullString `json:"deleted_at"`
	DiggerConfig         sql.NullString `json:"digger_config"`
	GithubInstallationId sql.NullInt64  `json:"github_installation_id"`
	Id                   sql.NullString `json:"id"`
	IsApproved           sql.NullBool   `json:"is_approved"`
	PlanStageId          sql.NullString `json:"plan_stage_id"`
	PrNumber             sql.NullInt64  `json:"pr_number"`
	ProjectName          sql.NullString `json:"project_name"`
	RepoId               sql.NullInt64  `json:"repo_id"`
	RunType              sql.NullString `json:"run_type"`
	Status               sql.NullString `json:"status"`
	Triggertype          sql.NullString `json:"triggertype"`
	UpdatedAt            sql.NullString `json:"updated_at"`
}

type PublicGithubAppInstallationLinksSelect struct {
	CreatedAt            string         `json:"created_at"`
	DeletedAt            sql.NullString `json:"deleted_at"`
	GithubInstallationId int64          `json:"github_installation_id"`
	Id                   string         `json:"id"`
	OrganizationId       string         `json:"organization_id"`
	Status               int16          `json:"status"`
	UpdatedAt            string         `json:"updated_at"`
}

type PublicGithubAppInstallationLinksInsert struct {
	CreatedAt            sql.NullString `json:"created_at"`
	DeletedAt            sql.NullString `json:"deleted_at"`
	GithubInstallationId int64          `json:"github_installation_id"`
	Id                   sql.NullString `json:"id"`
	OrganizationId       string         `json:"organization_id"`
	Status               int16          `json:"status"`
	UpdatedAt            sql.NullString `json:"updated_at"`
}

type PublicGithubAppInstallationLinksUpdate struct {
	CreatedAt            sql.NullString `json:"created_at"`
	DeletedAt            sql.NullString `json:"deleted_at"`
	GithubInstallationId sql.NullInt64  `json:"github_installation_id"`
	Id                   sql.NullString `json:"id"`
	OrganizationId       sql.NullString `json:"organization_id"`
	Status               sql.NullInt32  `json:"status"`
	UpdatedAt            sql.NullString `json:"updated_at"`
}

type PublicGithubAppInstallationsSelect struct {
	AccountId            int64          `json:"account_id"`
	CreatedAt            string         `json:"created_at"`
	DeletedAt            sql.NullString `json:"deleted_at"`
	GithubAppId          int64          `json:"github_app_id"`
	GithubInstallationId int64          `json:"github_installation_id"`
	Id                   string         `json:"id"`
	Login                string         `json:"login"`
	Repo                 sql.NullString `json:"repo"`
	Status               int64          `json:"status"`
	UpdatedAt            string         `json:"updated_at"`
}

type PublicGithubAppInstallationsInsert struct {
	AccountId            int64          `json:"account_id"`
	CreatedAt            sql.NullString `json:"created_at"`
	DeletedAt            sql.NullString `json:"deleted_at"`
	GithubAppId          int64          `json:"github_app_id"`
	GithubInstallationId int64          `json:"github_installation_id"`
	Id                   sql.NullString `json:"id"`
	Login                string         `json:"login"`
	Repo                 sql.NullString `json:"repo"`
	Status               int64          `json:"status"`
	UpdatedAt            sql.NullString `json:"updated_at"`
}

type PublicGithubAppInstallationsUpdate struct {
	AccountId            sql.NullInt64  `json:"account_id"`
	CreatedAt            sql.NullString `json:"created_at"`
	DeletedAt            sql.NullString `json:"deleted_at"`
	GithubAppId          sql.NullInt64  `json:"github_app_id"`
	GithubInstallationId sql.NullInt64  `json:"github_installation_id"`
	Id                   sql.NullString `json:"id"`
	Login                sql.NullString `json:"login"`
	Repo                 sql.NullString `json:"repo"`
	Status               sql.NullInt64  `json:"status"`
	UpdatedAt            sql.NullString `json:"updated_at"`
}

type PublicGithubAppsSelect struct {
	CreatedAt    string         `json:"created_at"`
	DeletedAt    sql.NullString `json:"deleted_at"`
	GithubAppUrl string         `json:"github_app_url"`
	GithubId     int64          `json:"github_id"`
	Id           string         `json:"id"`
	Name         string         `json:"name"`
	UpdatedAt    string         `json:"updated_at"`
}

type PublicGithubAppsInsert struct {
	CreatedAt    sql.NullString `json:"created_at"`
	DeletedAt    sql.NullString `json:"deleted_at"`
	GithubAppUrl string         `json:"github_app_url"`
	GithubId     int64          `json:"github_id"`
	Id           sql.NullString `json:"id"`
	Name         string         `json:"name"`
	UpdatedAt    sql.NullString `json:"updated_at"`
}

type PublicGithubAppsUpdate struct {
	CreatedAt    sql.NullString `json:"created_at"`
	DeletedAt    sql.NullString `json:"deleted_at"`
	GithubAppUrl sql.NullString `json:"github_app_url"`
	GithubId     sql.NullInt64  `json:"github_id"`
	Id           sql.NullString `json:"id"`
	Name         sql.NullString `json:"name"`
	UpdatedAt    sql.NullString `json:"updated_at"`
}
