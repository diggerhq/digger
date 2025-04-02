package models

import (
	"time"

	"github.com/diggerhq/digger/libs/scheduler"
)

func (db *Database) GetRepoCount(orgID uint) (int64, error) {
	var count int64

	thirtyDaysAgo := time.Now().AddDate(0, 0, -30)

	result := db.GormDB.Model(&Repo{}).
		Where("organisation_id = ? AND created_at >= ?", orgID, thirtyDaysAgo).
		Count(&count)

	if result.Error != nil {
		return 0, result.Error
	}

	return count, nil
}

func (db *Database) GetJobsCountThisMonth(orgID uint) (int64, error) {
	var count int64

	thirtyDaysAgo := time.Now().AddDate(0, 0, -30)

	result := db.GormDB.Model(DiggerJob{}).
		Joins("JOIN digger_batches ON digger_jobs.batch_id = digger_batches.id").
		Joins("JOIN github_app_installation_links ON digger_batches.github_installation_id = github_app_installation_links.github_installation_id").
		Joins("JOIN organisations ON github_app_installation_links.organisation_id = organisations.id").
		Where("digger_jobs.created_at >= ? AND organisations.id = ?", thirtyDaysAgo, orgID).
		Count(&count)

	if result.Error != nil {
		return 0, result.Error
	}

	return count, nil
}

func (db *Database) GetSuccessfulJobsCountThisMonth(orgID uint) (int64, error) {
	var count int64

	thirtyDaysAgo := time.Now().AddDate(0, 0, -30)

	result := db.GormDB.Model(DiggerJob{}).
		Joins("JOIN digger_batches ON digger_jobs.batch_id = digger_batches.id").
		Joins("JOIN github_app_installation_links ON digger_batches.github_installation_id = github_app_installation_links.github_installation_id").
		Joins("JOIN organisations ON github_app_installation_links.organisation_id = organisations.id").
		Where("digger_jobs.created_at >= ? AND organisations.id = ?", thirtyDaysAgo, orgID).
		Where("digger_jobs.status = ?", scheduler.DiggerJobSucceeded).
		Count(&count)

	if result.Error != nil {
		return 0, result.Error
	}

	return count, nil
}
