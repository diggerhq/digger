package models

import (
	"database/sql"
	"fmt"
	"github.com/diggerhq/digger/next/supa"
	"log"
)

// Note: we need to eventually move this file to gorm queries in order to make it easier for

func ToNullString(s string) sql.NullString {
	return sql.NullString{
		String: s,
		Valid:  true,
	}
}
func CreateRepoSupa(repoName string, orgId string) error {
	client, err := supa.GetClient()
	if err != nil {
		log.Printf("error creating supabase client for inseration: %v", err)
		return fmt.Errorf("could not create supabase client: %v", err)
	}

	_, cnt, err := client.From("repos").Select("id", "1", false).ExecuteTo()
	if cnt != 0 {
		return nil
	}
	payload := PublicReposInsert{
		Name:           repoName,
		OrganizationId: ToNullString(orgId),
		DiggerConfig:   ToNullString(""),
	}
	_, _, err = client.From("repos").Insert(payload, false, "DO NOTHING", "", "exact").Execute()
	if err != nil {
		log.Printf("could not insert repo record: %v", err)
		return fmt.Errorf("could not insert repo record: %v", err)
	}
	return nil
}
