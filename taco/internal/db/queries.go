package db

import (
	"gorm.io/gorm"
	"log"
)

func ListUnitsForUser(db *gorm.DB, userSubject string) ([]Unit, error) {
    var units []Unit
    
    err := db.Where("id IN (?)",
        db.Table("rule_units ru").
            Select("ru.unit_id").
            Joins("JOIN rules r ON ru.rule_id = r.id").
            Joins("JOIN role_permissions rp ON r.permission_id = rp.permission_id").  
            Joins("JOIN user_roles ur ON rp.role_id = ur.role_id").
            Joins("JOIN users u ON ur.user_id = u.id").
            Where("u.subject = ? AND r.effect = 'allow'", userSubject)).
        Preload("Tags").
        Find(&units).Error
        
    return units, err
}


// POC 
// Replace S3Store.List 
func ListAllUnits(db *gorm.DB, prefix string) ([]Unit, error) {
	log.Println("ListAllUnits", prefix)
    var units []Unit
    query := db.Preload("Tags")
    
    if prefix != "" {
        query = query.Where("name LIKE ?", prefix+"%")
    }
    
    return units, query.Find(&units).Error
}



// POC 
func FilterUnitIDsByUser(db *gorm.DB, userSubject string, unitIDs []string) ([]string, error) {
    log.Printf("FilterUnitIDsByUser: user=%s, checking %d units", userSubject, len(unitIDs))
    
    if len(unitIDs) == 0 {
        return []string{}, nil
    }
    
    var allowedUnitIDs []string
    
    // Super simple query using the flattened view!
    err := db.Table("user_unit_access").
        Select("unit_name").
        Where("user_subject = ?", userSubject).
        Where("unit_name IN ?", unitIDs).
        Pluck("unit_name", &allowedUnitIDs).Error
    
    log.Printf("User %s has access to %d/%d units", userSubject, len(allowedUnitIDs), len(unitIDs))
    return allowedUnitIDs, err
}

func ListAllUnitsWithPrefix(db *gorm.DB, prefix string) ([]Unit, error) {
    var units []Unit
    query := db.Preload("Tags")
    
    if prefix != "" {
        query = query.Where("name LIKE ?", prefix+"%")
    }
    
    return units, query.Find(&units).Error
}



/// POC - write to db example 
// Sync functions to keep database in sync with storage operations
func SyncCreateUnit(db *gorm.DB, unitName string) error {
    unit := Unit{Name: unitName}
    return db.FirstOrCreate(&unit, Unit{Name: unitName}).Error
}

func SyncDeleteUnit(db *gorm.DB, unitName string) error {
    return db.Where("name = ?", unitName).Delete(&Unit{}).Error
}

func SyncUnitExists(db *gorm.DB, unitName string) error {
    unit := Unit{Name: unitName}
    return db.FirstOrCreate(&unit, Unit{Name: unitName}).Error
}