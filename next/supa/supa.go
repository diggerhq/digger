package supa

import (
	"fmt"
	"github.com/supabase-community/supabase-go"
	"os"
)

var client *supabase.Client = nil

func GetClient() (*supabase.Client, error) {
	ApiUrl := os.Getenv("DIGGER_SUPABASE_API_URL")
	ApiKey := os.Getenv("DIGGER_SUPABASE_API_KEY")
	var err error
	client, err = supabase.NewClient(ApiUrl, ApiKey, nil)
	if err != nil {
		fmt.Println("cannot initialize supabase client", err)
		return nil, fmt.Errorf("could not create supabase client: %v", err)
	}
	return client, err
}
