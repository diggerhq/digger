module github.com/diggerhq/digger/next

go 1.22.4

replace github.com/diggerhq/digger/backend => ../backend

replace github.com/diggerhq/digger/libs => ../libs

require (
	github.com/google/uuid v1.6.0 // indirect
	github.com/supabase-community/functions-go v0.0.0-20220927045802-22373e6cb51d // indirect
	github.com/supabase-community/gotrue-go v1.2.0 // indirect
	github.com/supabase-community/postgrest-go v0.0.11 // indirect
	github.com/supabase-community/storage-go v0.7.0 // indirect
	github.com/supabase-community/supabase-go v0.0.4 // indirect
	github.com/tomnomnom/linkheader v0.0.0-20180905144013-02ca5825eb80 // indirect
)
