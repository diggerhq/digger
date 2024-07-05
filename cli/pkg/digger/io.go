package digger

import (
	"fmt"
	"github.com/diggerhq/digger/libs/ci"
	"github.com/diggerhq/digger/libs/scheduler"
	"log"
)

func UpdateAggregateStatus(batch *scheduler.SerializedBatch, prService ci.PullRequestService) error {
	// TODO: Introduce batch-level
	isPlan, err := batch.IsPlan()
	if err != nil {
		log.Printf("failed to get batch job plan/apply status: %v", err)
		return fmt.Errorf("failed to get batch job plan/apply status: %v", err)
	}

	if isPlan {
		prService.SetStatus(batch.PrNumber, batch.ToStatusCheck(), "digger/plan")
		prService.SetStatus(batch.PrNumber, "pending", "digger/apply")
	} else {
		prService.SetStatus(batch.PrNumber, "success", "digger/plan")
		prService.SetStatus(batch.PrNumber, batch.ToStatusCheck(), "digger/apply")
	}
	return nil
}
