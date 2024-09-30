package dbmodels

import (
	"github.com/diggerhq/digger/ee/drift/model"
	"github.com/google/uuid"
	"log"
	"time"
)

const (
	AccessPolicyType = "access"
	AdminPolicyType  = "admin"
	CliJobAccessType = "cli_access"
)

func (db *Database) CreateDiggerJobToken(organisationId string) (*model.DiggerCiJobToken, error) {

	// create a digger job token
	// prefixing token to make easier to retire this type of tokens later
	token := "cli:" + uuid.New().String()
	jobToken := &model.DiggerCiJobToken{
		Value:          token,
		OrganisationID: organisationId,
		Type:           CliJobAccessType,
		Expiry:         time.Now().Add(time.Hour * 2), // some jobs can take >30 mins (k8s cluster)
	}
	err := db.GormDB.Create(jobToken).Error
	if err != nil {
		log.Printf("failed to create token: %v", err)
		return nil, err
	}
	return jobToken, nil
}
