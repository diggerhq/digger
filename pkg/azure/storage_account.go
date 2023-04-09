package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/data/aztables"
)

const (
	TABLE_NAME = "DIGGERLOCK"
)

var (
	SERVICE_URL_FORMAT = "https://%s.table.core.windows.net"
)

type StorageAccount struct {
	tableClient *aztables.Client
	svcClient   *aztables.ServiceClient
}

func getServiceClient() (*aztables.ServiceClient, error) {
	if key := (os.Getenv("DIGGER_AZURE_SHARED_KEY")); key != "" {
		saName := os.Getenv("DIGGER_AZURE_SA_NAME")
		if saName == "" {
			return nil, fmt.Errorf("you must set 'DIGGER_AZURE_SA_NAME' environment variable when using shared key authentication")
		}

		sharedCreds, err := aztables.NewSharedKeyCredential(saName, key)
		if err != nil {
			return nil, fmt.Errorf("could not create shared key credentials: %v", err)
		}

		serviceURL := getServiceURL(saName)
		svcClient, err := aztables.NewServiceClientWithSharedKey(serviceURL, sharedCreds, nil)
		if err != nil {
			return nil, fmt.Errorf("could not create service client with shared key authentication: %v", err)
		}
		return svcClient, nil
	}

	if connStr := os.Getenv("DIGGER_AZURE_CONNECTION_STRING"); connStr != "" {
		svcClient, err := aztables.NewServiceClientFromConnectionString(connStr, nil)
		if err != nil {
			return nil, fmt.Errorf("could not create service client with connection string authentication: %v", err)
		}
		return svcClient, err
	}

	if tenantId := os.Getenv("DIGGER_AZURE_TENANT_ID"); tenantId != "" {
		clientId := os.Getenv("DIGGER_AZURE_CLIENT_ID")
		secret := os.Getenv("DIGGER_AZURE_CLIENT_SECRET")

		if clientId == "" || secret == "" {
			return nil, fmt.Errorf("you must set 'DIGGER_AZURE_CLIENT_ID' and 'DIGGER_AZURE_CLIENT_SECRET' when using client secret authentication")
		}

		saName := os.Getenv("DIGGER_AZURE_SA_NAME")
		if saName == "" {
			return nil, fmt.Errorf("you must set 'DIGGER_AZURE_SA_NAME' environment variable when using client secret authentication")
		}

		serviceURL := getServiceURL(saName)
		cred, err := azidentity.NewClientSecretCredential(tenantId, clientId, secret, nil)
		if err != nil {
			return nil, fmt.Errorf("could not create create client secret credential: %v", err)
		}
		svcClient, err := aztables.NewServiceClient(serviceURL, cred, nil)
		if err != nil {
			return nil, fmt.Errorf("could not create service client with client secret authentication: %v", err)
		}
		return svcClient, nil
	}

	return nil, fmt.Errorf("could not initialize service client, because no authentication method was found")

}

func NewStorageAccountLock() (*StorageAccount, error) {
	svcClient, err := getServiceClient()
	if err != nil {
		return nil, err
	}

	return &StorageAccount{
		svcClient:   svcClient,
		tableClient: svcClient.NewClient(TABLE_NAME),
	}, nil
}

func (sal *StorageAccount) Lock(transactionId int, resource string) (bool, error) {
	err := sal.createTableIfNotExists()
	if err != nil {
		return false, err
	}

	entity := aztables.EDMEntity{
		Properties: map[string]interface{}{
			"transaction_id": transactionId,
		},
		Entity: aztables.Entity{
			PartitionKey: "digger",
			RowKey:       resource,
		},
	}
	b, err := json.Marshal(entity)
	if err != nil {
		return false, fmt.Errorf("could not marshall entity: %v", err)
	}

	_, err = sal.tableClient.AddEntity(context.Background(), b, nil)
	if err != nil {
		if strings.Contains(err.Error(), "EntityAlreadyExists") {
			return false, nil
		}
		return false, fmt.Errorf("could not add entity: \n%v", err)
	}

	return true, nil
}

func (sal *StorageAccount) Unlock(resource string) (bool, error) {
	_, err := sal.tableClient.DeleteEntity(context.Background(), "digger", resource, nil)
	if err != nil {
		return false, fmt.Errorf("could not delete lock: %v", err)
	}

	return true, nil
}

func (sal *StorageAccount) GetLock(resource string) (*int, error) {
	filterQuery := fmt.Sprintf("PartitionKey eq 'digger' and RowKey eq '%s'", resource)
	selectQuery := "RowKey,PartitionKey,transaction_id"
	listOpts := aztables.ListEntitiesOptions{
		Filter: &filterQuery,
		Select: &selectQuery,
	}

	entitiesPager := sal.tableClient.NewListEntitiesPager(&listOpts)
	for entitiesPager.More() {
		res, err := entitiesPager.NextPage(context.Background())
		if err != nil {
			return nil, fmt.Errorf("could not retrieve the entities: %v", err)
		}

		for _, e := range res.Entities {
			var entity aztables.EDMEntity
			err := json.Unmarshal(e, &entity)
			if err != nil {
				return new(int), fmt.Errorf("could not unmarshall entity: %v", err)
			}

			transactionId := int(entity.Properties["transaction_id"].(int32))
			return &transactionId, nil
		}
	}

	// Lock doesn't exist
	return nil, nil
}

func (sal *StorageAccount) createTableIfNotExists() error {
	exists, err := sal.isTableExists(TABLE_NAME)
	if err != nil {
		return err
	}

	if exists {
		return nil
	}

	// Table doesn't exist, we create it
	_, err = sal.tableClient.CreateTable(context.TODO(), nil)
	if err != nil {
		return fmt.Errorf("could not create table: %v", err)
	}

	return nil
}

func (sal *StorageAccount) isTableExists(table string) (bool, error) {
	tablesPager := sal.svcClient.NewListTablesPager(nil)
	for tablesPager.More() {
		res, err := tablesPager.NextPage(context.Background())
		if err != nil {
			return false, fmt.Errorf("could not retrieve the tables: %v", err)
		}

		for _, t := range res.Tables {
			if *t.Name == table {
				return true, nil
			}
		}
	}

	return false, nil
}

func getServiceURL(saName string) string {
	return fmt.Sprintf(SERVICE_URL_FORMAT, saName)
}

// func (sal *StorageAccount) Lock(transactionId int, resource string) (bool, error) {
// 	options := aztables.ListEntitiesOptions{}
// 	entitiesPager := sal.tableClient.NewListEntitiesPager(&options)

// 	for entitiesPager.More() {
// 		marshEntites, err := entitiesPager.NextPage(context.Background())
// 		if err != nil {
// 			return false, fmt.Errorf("could not advance to next page while retrieving entities: %v", err)
// 		}

// 		var entity aztables.EDMEntity
// 		json.Unmarshal(marshEntites.Entities, &entity)
// 	}
// 	return true, nil
// }
