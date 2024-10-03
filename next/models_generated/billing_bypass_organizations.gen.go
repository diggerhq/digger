// Code generated by gorm.io/gen. DO NOT EDIT.
// Code generated by gorm.io/gen. DO NOT EDIT.
// Code generated by gorm.io/gen. DO NOT EDIT.

package models_generated

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"

	"gorm.io/gen"
	"gorm.io/gen/field"

	"gorm.io/plugin/dbresolver"

	"github.com/diggerhq/digger/next/model"
)

func newBillingBypassOrganization(db *gorm.DB, opts ...gen.DOOption) billingBypassOrganization {
	_billingBypassOrganization := billingBypassOrganization{}

	_billingBypassOrganization.billingBypassOrganizationDo.UseDB(db, opts...)
	_billingBypassOrganization.billingBypassOrganizationDo.UseModel(&model.BillingBypassOrganization{})

	tableName := _billingBypassOrganization.billingBypassOrganizationDo.TableName()
	_billingBypassOrganization.ALL = field.NewAsterisk(tableName)
	_billingBypassOrganization.ID = field.NewString(tableName, "id")
	_billingBypassOrganization.CreatedAt = field.NewTime(tableName, "created_at")

	_billingBypassOrganization.fillFieldMap()

	return _billingBypassOrganization
}

type billingBypassOrganization struct {
	billingBypassOrganizationDo

	ALL       field.Asterisk
	ID        field.String
	CreatedAt field.Time

	fieldMap map[string]field.Expr
}

func (b billingBypassOrganization) Table(newTableName string) *billingBypassOrganization {
	b.billingBypassOrganizationDo.UseTable(newTableName)
	return b.updateTableName(newTableName)
}

func (b billingBypassOrganization) As(alias string) *billingBypassOrganization {
	b.billingBypassOrganizationDo.DO = *(b.billingBypassOrganizationDo.As(alias).(*gen.DO))
	return b.updateTableName(alias)
}

func (b *billingBypassOrganization) updateTableName(table string) *billingBypassOrganization {
	b.ALL = field.NewAsterisk(table)
	b.ID = field.NewString(table, "id")
	b.CreatedAt = field.NewTime(table, "created_at")

	b.fillFieldMap()

	return b
}

func (b *billingBypassOrganization) GetFieldByName(fieldName string) (field.OrderExpr, bool) {
	_f, ok := b.fieldMap[fieldName]
	if !ok || _f == nil {
		return nil, false
	}
	_oe, ok := _f.(field.OrderExpr)
	return _oe, ok
}

func (b *billingBypassOrganization) fillFieldMap() {
	b.fieldMap = make(map[string]field.Expr, 2)
	b.fieldMap["id"] = b.ID
	b.fieldMap["created_at"] = b.CreatedAt
}

func (b billingBypassOrganization) clone(db *gorm.DB) billingBypassOrganization {
	b.billingBypassOrganizationDo.ReplaceConnPool(db.Statement.ConnPool)
	return b
}

func (b billingBypassOrganization) replaceDB(db *gorm.DB) billingBypassOrganization {
	b.billingBypassOrganizationDo.ReplaceDB(db)
	return b
}

type billingBypassOrganizationDo struct{ gen.DO }

type IBillingBypassOrganizationDo interface {
	gen.SubQuery
	Debug() IBillingBypassOrganizationDo
	WithContext(ctx context.Context) IBillingBypassOrganizationDo
	WithResult(fc func(tx gen.Dao)) gen.ResultInfo
	ReplaceDB(db *gorm.DB)
	ReadDB() IBillingBypassOrganizationDo
	WriteDB() IBillingBypassOrganizationDo
	As(alias string) gen.Dao
	Session(config *gorm.Session) IBillingBypassOrganizationDo
	Columns(cols ...field.Expr) gen.Columns
	Clauses(conds ...clause.Expression) IBillingBypassOrganizationDo
	Not(conds ...gen.Condition) IBillingBypassOrganizationDo
	Or(conds ...gen.Condition) IBillingBypassOrganizationDo
	Select(conds ...field.Expr) IBillingBypassOrganizationDo
	Where(conds ...gen.Condition) IBillingBypassOrganizationDo
	Order(conds ...field.Expr) IBillingBypassOrganizationDo
	Distinct(cols ...field.Expr) IBillingBypassOrganizationDo
	Omit(cols ...field.Expr) IBillingBypassOrganizationDo
	Join(table schema.Tabler, on ...field.Expr) IBillingBypassOrganizationDo
	LeftJoin(table schema.Tabler, on ...field.Expr) IBillingBypassOrganizationDo
	RightJoin(table schema.Tabler, on ...field.Expr) IBillingBypassOrganizationDo
	Group(cols ...field.Expr) IBillingBypassOrganizationDo
	Having(conds ...gen.Condition) IBillingBypassOrganizationDo
	Limit(limit int) IBillingBypassOrganizationDo
	Offset(offset int) IBillingBypassOrganizationDo
	Count() (count int64, err error)
	Scopes(funcs ...func(gen.Dao) gen.Dao) IBillingBypassOrganizationDo
	Unscoped() IBillingBypassOrganizationDo
	Create(values ...*model.BillingBypassOrganization) error
	CreateInBatches(values []*model.BillingBypassOrganization, batchSize int) error
	Save(values ...*model.BillingBypassOrganization) error
	First() (*model.BillingBypassOrganization, error)
	Take() (*model.BillingBypassOrganization, error)
	Last() (*model.BillingBypassOrganization, error)
	Find() ([]*model.BillingBypassOrganization, error)
	FindInBatch(batchSize int, fc func(tx gen.Dao, batch int) error) (results []*model.BillingBypassOrganization, err error)
	FindInBatches(result *[]*model.BillingBypassOrganization, batchSize int, fc func(tx gen.Dao, batch int) error) error
	Pluck(column field.Expr, dest interface{}) error
	Delete(...*model.BillingBypassOrganization) (info gen.ResultInfo, err error)
	Update(column field.Expr, value interface{}) (info gen.ResultInfo, err error)
	UpdateSimple(columns ...field.AssignExpr) (info gen.ResultInfo, err error)
	Updates(value interface{}) (info gen.ResultInfo, err error)
	UpdateColumn(column field.Expr, value interface{}) (info gen.ResultInfo, err error)
	UpdateColumnSimple(columns ...field.AssignExpr) (info gen.ResultInfo, err error)
	UpdateColumns(value interface{}) (info gen.ResultInfo, err error)
	UpdateFrom(q gen.SubQuery) gen.Dao
	Attrs(attrs ...field.AssignExpr) IBillingBypassOrganizationDo
	Assign(attrs ...field.AssignExpr) IBillingBypassOrganizationDo
	Joins(fields ...field.RelationField) IBillingBypassOrganizationDo
	Preload(fields ...field.RelationField) IBillingBypassOrganizationDo
	FirstOrInit() (*model.BillingBypassOrganization, error)
	FirstOrCreate() (*model.BillingBypassOrganization, error)
	FindByPage(offset int, limit int) (result []*model.BillingBypassOrganization, count int64, err error)
	ScanByPage(result interface{}, offset int, limit int) (count int64, err error)
	Scan(result interface{}) (err error)
	Returning(value interface{}, columns ...string) IBillingBypassOrganizationDo
	UnderlyingDB() *gorm.DB
	schema.Tabler
}

func (b billingBypassOrganizationDo) Debug() IBillingBypassOrganizationDo {
	return b.withDO(b.DO.Debug())
}

func (b billingBypassOrganizationDo) WithContext(ctx context.Context) IBillingBypassOrganizationDo {
	return b.withDO(b.DO.WithContext(ctx))
}

func (b billingBypassOrganizationDo) ReadDB() IBillingBypassOrganizationDo {
	return b.Clauses(dbresolver.Read)
}

func (b billingBypassOrganizationDo) WriteDB() IBillingBypassOrganizationDo {
	return b.Clauses(dbresolver.Write)
}

func (b billingBypassOrganizationDo) Session(config *gorm.Session) IBillingBypassOrganizationDo {
	return b.withDO(b.DO.Session(config))
}

func (b billingBypassOrganizationDo) Clauses(conds ...clause.Expression) IBillingBypassOrganizationDo {
	return b.withDO(b.DO.Clauses(conds...))
}

func (b billingBypassOrganizationDo) Returning(value interface{}, columns ...string) IBillingBypassOrganizationDo {
	return b.withDO(b.DO.Returning(value, columns...))
}

func (b billingBypassOrganizationDo) Not(conds ...gen.Condition) IBillingBypassOrganizationDo {
	return b.withDO(b.DO.Not(conds...))
}

func (b billingBypassOrganizationDo) Or(conds ...gen.Condition) IBillingBypassOrganizationDo {
	return b.withDO(b.DO.Or(conds...))
}

func (b billingBypassOrganizationDo) Select(conds ...field.Expr) IBillingBypassOrganizationDo {
	return b.withDO(b.DO.Select(conds...))
}

func (b billingBypassOrganizationDo) Where(conds ...gen.Condition) IBillingBypassOrganizationDo {
	return b.withDO(b.DO.Where(conds...))
}

func (b billingBypassOrganizationDo) Order(conds ...field.Expr) IBillingBypassOrganizationDo {
	return b.withDO(b.DO.Order(conds...))
}

func (b billingBypassOrganizationDo) Distinct(cols ...field.Expr) IBillingBypassOrganizationDo {
	return b.withDO(b.DO.Distinct(cols...))
}

func (b billingBypassOrganizationDo) Omit(cols ...field.Expr) IBillingBypassOrganizationDo {
	return b.withDO(b.DO.Omit(cols...))
}

func (b billingBypassOrganizationDo) Join(table schema.Tabler, on ...field.Expr) IBillingBypassOrganizationDo {
	return b.withDO(b.DO.Join(table, on...))
}

func (b billingBypassOrganizationDo) LeftJoin(table schema.Tabler, on ...field.Expr) IBillingBypassOrganizationDo {
	return b.withDO(b.DO.LeftJoin(table, on...))
}

func (b billingBypassOrganizationDo) RightJoin(table schema.Tabler, on ...field.Expr) IBillingBypassOrganizationDo {
	return b.withDO(b.DO.RightJoin(table, on...))
}

func (b billingBypassOrganizationDo) Group(cols ...field.Expr) IBillingBypassOrganizationDo {
	return b.withDO(b.DO.Group(cols...))
}

func (b billingBypassOrganizationDo) Having(conds ...gen.Condition) IBillingBypassOrganizationDo {
	return b.withDO(b.DO.Having(conds...))
}

func (b billingBypassOrganizationDo) Limit(limit int) IBillingBypassOrganizationDo {
	return b.withDO(b.DO.Limit(limit))
}

func (b billingBypassOrganizationDo) Offset(offset int) IBillingBypassOrganizationDo {
	return b.withDO(b.DO.Offset(offset))
}

func (b billingBypassOrganizationDo) Scopes(funcs ...func(gen.Dao) gen.Dao) IBillingBypassOrganizationDo {
	return b.withDO(b.DO.Scopes(funcs...))
}

func (b billingBypassOrganizationDo) Unscoped() IBillingBypassOrganizationDo {
	return b.withDO(b.DO.Unscoped())
}

func (b billingBypassOrganizationDo) Create(values ...*model.BillingBypassOrganization) error {
	if len(values) == 0 {
		return nil
	}
	return b.DO.Create(values)
}

func (b billingBypassOrganizationDo) CreateInBatches(values []*model.BillingBypassOrganization, batchSize int) error {
	return b.DO.CreateInBatches(values, batchSize)
}

// Save : !!! underlying implementation is different with GORM
// The method is equivalent to executing the statement: db.Clauses(clause.OnConflict{UpdateAll: true}).Create(values)
func (b billingBypassOrganizationDo) Save(values ...*model.BillingBypassOrganization) error {
	if len(values) == 0 {
		return nil
	}
	return b.DO.Save(values)
}

func (b billingBypassOrganizationDo) First() (*model.BillingBypassOrganization, error) {
	if result, err := b.DO.First(); err != nil {
		return nil, err
	} else {
		return result.(*model.BillingBypassOrganization), nil
	}
}

func (b billingBypassOrganizationDo) Take() (*model.BillingBypassOrganization, error) {
	if result, err := b.DO.Take(); err != nil {
		return nil, err
	} else {
		return result.(*model.BillingBypassOrganization), nil
	}
}

func (b billingBypassOrganizationDo) Last() (*model.BillingBypassOrganization, error) {
	if result, err := b.DO.Last(); err != nil {
		return nil, err
	} else {
		return result.(*model.BillingBypassOrganization), nil
	}
}

func (b billingBypassOrganizationDo) Find() ([]*model.BillingBypassOrganization, error) {
	result, err := b.DO.Find()
	return result.([]*model.BillingBypassOrganization), err
}

func (b billingBypassOrganizationDo) FindInBatch(batchSize int, fc func(tx gen.Dao, batch int) error) (results []*model.BillingBypassOrganization, err error) {
	buf := make([]*model.BillingBypassOrganization, 0, batchSize)
	err = b.DO.FindInBatches(&buf, batchSize, func(tx gen.Dao, batch int) error {
		defer func() { results = append(results, buf...) }()
		return fc(tx, batch)
	})
	return results, err
}

func (b billingBypassOrganizationDo) FindInBatches(result *[]*model.BillingBypassOrganization, batchSize int, fc func(tx gen.Dao, batch int) error) error {
	return b.DO.FindInBatches(result, batchSize, fc)
}

func (b billingBypassOrganizationDo) Attrs(attrs ...field.AssignExpr) IBillingBypassOrganizationDo {
	return b.withDO(b.DO.Attrs(attrs...))
}

func (b billingBypassOrganizationDo) Assign(attrs ...field.AssignExpr) IBillingBypassOrganizationDo {
	return b.withDO(b.DO.Assign(attrs...))
}

func (b billingBypassOrganizationDo) Joins(fields ...field.RelationField) IBillingBypassOrganizationDo {
	for _, _f := range fields {
		b = *b.withDO(b.DO.Joins(_f))
	}
	return &b
}

func (b billingBypassOrganizationDo) Preload(fields ...field.RelationField) IBillingBypassOrganizationDo {
	for _, _f := range fields {
		b = *b.withDO(b.DO.Preload(_f))
	}
	return &b
}

func (b billingBypassOrganizationDo) FirstOrInit() (*model.BillingBypassOrganization, error) {
	if result, err := b.DO.FirstOrInit(); err != nil {
		return nil, err
	} else {
		return result.(*model.BillingBypassOrganization), nil
	}
}

func (b billingBypassOrganizationDo) FirstOrCreate() (*model.BillingBypassOrganization, error) {
	if result, err := b.DO.FirstOrCreate(); err != nil {
		return nil, err
	} else {
		return result.(*model.BillingBypassOrganization), nil
	}
}

func (b billingBypassOrganizationDo) FindByPage(offset int, limit int) (result []*model.BillingBypassOrganization, count int64, err error) {
	result, err = b.Offset(offset).Limit(limit).Find()
	if err != nil {
		return
	}

	if size := len(result); 0 < limit && 0 < size && size < limit {
		count = int64(size + offset)
		return
	}

	count, err = b.Offset(-1).Limit(-1).Count()
	return
}

func (b billingBypassOrganizationDo) ScanByPage(result interface{}, offset int, limit int) (count int64, err error) {
	count, err = b.Count()
	if err != nil {
		return
	}

	err = b.Offset(offset).Limit(limit).Scan(result)
	return
}

func (b billingBypassOrganizationDo) Scan(result interface{}) (err error) {
	return b.DO.Scan(result)
}

func (b billingBypassOrganizationDo) Delete(models ...*model.BillingBypassOrganization) (result gen.ResultInfo, err error) {
	return b.DO.Delete(models)
}

func (b *billingBypassOrganizationDo) withDO(do gen.Dao) *billingBypassOrganizationDo {
	b.DO = *do.(*gen.DO)
	return b
}