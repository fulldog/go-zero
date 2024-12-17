package crmx

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Masterminds/squirrel"
	"github.com/foursking/btype"
	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"golang.org/x/sync/errgroup"
	"reflect"
	"strings"
	"sync"
	"time"
)

const (
	defaultValue = "defaultValue"
	columnType   = "columnType"
	isNullAble   = "isNullAble"
	columnTag    = "db"
	nullAble     = "YES"
	DateTime     = "2006-01-02 15:04:05"
	DateOnly     = "2006-01-02"
)

var (
	DiyDbConn sqlx.SqlConn
	jsonNull  = []byte("null")
)

type Repository struct {
	conn sqlx.SqlConn
	ctx  context.Context
	list []func(ctx context.Context, conn sqlx.Session) error //前置事务
	last []func(ctx context.Context, conn sqlx.Session) error //后置事务  该组操作会被放到事务最后执行
	lock sync.Mutex
}

func NewRepository(ctx context.Context, conn sqlx.SqlConn) *Repository {
	if ctx == nil {
		ctx = context.Background()
	}
	return &Repository{
		conn: conn,
		ctx:  ctx,
	}
}

func (r *Repository) AddLast(fn ...func(ctx context.Context, session sqlx.Session) error) {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.last = append(r.last, fn...)
}

func (r *Repository) Add(fn ...func(ctx context.Context, session sqlx.Session) error) {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.list = append(r.list, fn...)
}

func (r *Repository) Run() error {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.list = append(r.list, r.last...)
	if len(r.list) == 0 {
		return nil
	}
	err := r.conn.TransactCtx(r.ctx, func(ctx context.Context, session sqlx.Session) error {
		for _, fn := range r.list {
			err := fn(ctx, session)
			if err != nil {
				return err
			}
		}
		return nil
	})
	return err
}

// RunGo 协程方式 注意需要注意db是否支持并行事务
func (r *Repository) RunGo() error {
	r.lock.Lock()
	defer r.lock.Unlock()
	if len(r.list) == 0 {
		return nil
	}
	err := r.conn.TransactCtx(r.ctx, func(ctx context.Context, session sqlx.Session) error {
		group := &errgroup.Group{}
		for _, fn := range r.list {
			fnx := fn
			group.Go(func() error {
				err := fnx(ctx, session)
				if err != nil {
					return err
				}
				return err
			})
		}
		return group.Wait()
	})
	return err
}

type baseModelFace[T any] interface {
	FindOneByPrimary(id int) (*T, error)
	Delete2(id int) error
	Insert2(data *T) (sql.Result, error)
	Update2(data *T) (sql.Result, error)
	UpdateSlice(fields []string, data *T) (sql.Result, error)
	UpdateWhere(data *T, where func(builder squirrel.UpdateBuilder) squirrel.UpdateBuilder) (sql.Result, error)
	UpdateMapWhere(data map[string]interface{}, where func(builder squirrel.UpdateBuilder) squirrel.UpdateBuilder) (sql.Result, error)
	FindRowsByWhere(fields []string, fn func(builder squirrel.SelectBuilder) squirrel.SelectBuilder) ([]*T, error)
	FindOneByWhere(fields []string, fn func(builder squirrel.SelectBuilder) squirrel.SelectBuilder) (*T, error)
	GetTableNameBySuffix() string
	QueryRawRow(data interface{}, query string, values ...interface{}) error
	QueryRawRows(data interface{}, query string, values ...interface{}) error
	SetCtx(ctx context.Context) *baseModel[T]
	SetConn(conn sqlx.SqlConn) *baseModel[T]
	SetAutoIncrKey(autoIncrKey string) *baseModel[T]
	SetSuffix(s string) *baseModel[T]
	TransUpdate(session sqlx.Session, data *T, where func(builder squirrel.UpdateBuilder) squirrel.UpdateBuilder) (sql.Result, error)
	TransUpdateMap(session sqlx.Session, data map[string]interface{}, where func(builder squirrel.UpdateBuilder) squirrel.UpdateBuilder) (sql.Result, error)
	TransUpdateSlice(session sqlx.Session, fields []string, data *T, where func(builder squirrel.UpdateBuilder) squirrel.UpdateBuilder) (sql.Result, error)
	TransInsert(session sqlx.Session, data *T) (sql.Result, error)
	TransInsertBatch(session sqlx.Session, data []*T) (sql.Result, error)
	Count(fn func(builder squirrel.SelectBuilder) squirrel.SelectBuilder) (int, error)
	Exist(fn func(builder squirrel.SelectBuilder) squirrel.SelectBuilder) bool
	GetConn() sqlx.SqlConn
	UpdateBuilder(where func(builder squirrel.UpdateBuilder) squirrel.UpdateBuilder) (sql.Result, error)
	TransUpdateBuilder(session sqlx.Session, where func(builder squirrel.UpdateBuilder) squirrel.UpdateBuilder) (sql.Result, error)
}

func NewBaseModel[T any](table string, autoIncrKey string) baseModelFace[T] {
	return &baseModel[T]{
		tableName:   table,
		autoIncrKey: autoIncrKey,
		ctx:         context.Background(),
	}
}

type baseModel[T any] struct {
	conn        sqlx.SqlConn
	ctx         context.Context
	tableName   string
	autoIncrKey string
	suffix      string
}

func (b *baseModel[T]) UpdateBuilder(where func(builder squirrel.UpdateBuilder) squirrel.UpdateBuilder) (sql.Result, error) {
	query, vals, _ := where(squirrel.Update(b.GetTableNameBySuffix())).ToSql()
	return b.conn.ExecCtx(b.ctx, query, vals...)
}
func (b *baseModel[T]) TransUpdateBuilder(session sqlx.Session, where func(builder squirrel.UpdateBuilder) squirrel.UpdateBuilder) (sql.Result, error) {
	query, vals, _ := where(squirrel.Update(b.GetTableNameBySuffix())).ToSql()
	return session.ExecCtx(b.ctx, query, vals...)
}

func (b *baseModel[T]) GetConn() sqlx.SqlConn {
	return b.conn
}

func (b *baseModel[T]) FindOneByPrimary(id int) (*T, error) {
	var t []*T
	query := fmt.Sprintf("select %s from %s where `%s` = ? limit 1", "*", b.GetTableNameBySuffix(), b.autoIncrKey)
	err := b.conn.QueryRowsCtx(b.ctx, &t, query, id)
	if len(t) > 0 {
		return t[0], nil
	}
	return nil, err
}

func (b *baseModel[T]) Delete2(id int) error {
	query := fmt.Sprintf("delete from %s where `%s` = ?", b.GetTableNameBySuffix(), b.autoIncrKey)
	_, err := b.conn.ExecCtx(b.ctx, query, id)
	return err
}

func (b *baseModel[T]) Insert2(data *T) (sql.Result, error) {
	columns, values := b.buildDataInsert(data)
	query, vals, _ := squirrel.Insert(b.GetTableNameBySuffix()).Columns(columns...).Values(values[0]...).ToSql()
	return b.conn.ExecCtx(b.ctx, query, vals...)
}

// Update2 Update model 更新，忽略0值，如果需更新为0值，请使用map方式
func (b *baseModel[T]) Update2(data *T) (sql.Result, error) {
	rafVal := reflect.ValueOf(data).Elem()
	rafTag := reflect.TypeOf(data).Elem()
	var values = make(map[string]interface{}, rafTag.NumField())
	for i := 0; i < rafVal.NumField(); i++ {
		if rafVal.Field(i).IsZero() {
			continue
		}
		tag := rafTag.Field(i).Tag.Get(columnTag)
		if tag == "" {
			tag = rafTag.Field(i).Name
		}
		values[tag] = rafVal.Field(i).Interface()
	}
	var idx = values[b.autoIncrKey]
	delete(values, b.autoIncrKey)
	query, vals, _ := squirrel.Update(b.GetTableNameBySuffix()).SetMap(values).Where(squirrel.Eq{b.autoIncrKey: idx}).ToSql()
	return b.conn.ExecCtx(b.ctx, query, vals...)
}

// UpdateSlice 指定更新字段
func (b *baseModel[T]) UpdateSlice(fields []string, data *T) (sql.Result, error) {
	rafVal := reflect.ValueOf(data).Elem()
	rafTag := reflect.TypeOf(data).Elem()
	var values = make(map[string]interface{}, len(fields))
	var field2name = make(map[string]string, len(fields))
	for j := 0; j < rafTag.NumField(); j++ {
		field2name[rafTag.Field(j).Tag.Get(columnTag)] = rafTag.Field(j).Name
	}
	for i := 0; i < len(fields); i++ {
		values[fields[i]] = rafVal.FieldByName(field2name[fields[i]]).Interface()
	}
	var idx = values[b.autoIncrKey]
	delete(values, b.autoIncrKey)
	query, vals, _ := squirrel.Update(b.GetTableNameBySuffix()).SetMap(values).Where(squirrel.Eq{b.autoIncrKey: idx}).ToSql()
	return b.conn.ExecCtx(b.ctx, query, vals...)
}

// UpdateWhere model 更新，忽略0值，如果需更新为0值，请使用map方式
func (b *baseModel[T]) UpdateWhere(data *T, where func(builder squirrel.UpdateBuilder) squirrel.UpdateBuilder) (sql.Result, error) {
	if where == nil {
		return nil, errors.New("where must be set")
	}
	rafVal := reflect.ValueOf(data).Elem()
	rafTag := reflect.TypeOf(data).Elem()
	var (
		values = make(map[string]interface{}, rafTag.NumField())
	)

	for i := 0; i < rafVal.NumField(); i++ {
		if rafVal.Field(i).IsZero() {
			continue
		}
		tag := rafTag.Field(i).Tag.Get(columnTag)
		if tag == "" {
			tag = rafTag.Field(i).Name
		}
		if tag != b.autoIncrKey {
			values[tag] = rafVal.Field(i).Interface()
		}
	}
	query, vals, _ := where(squirrel.Update(b.GetTableNameBySuffix()).SetMap(values)).ToSql()
	return b.conn.ExecCtx(b.ctx, query, vals...)
}

func (b *baseModel[T]) UpdateMapWhere(data map[string]interface{}, where func(builder squirrel.UpdateBuilder) squirrel.UpdateBuilder) (sql.Result, error) {
	if where == nil || data == nil {
		return nil, errors.New("where/data must be set")
	}
	query, vals, _ := where(squirrel.Update(b.GetTableNameBySuffix()).SetMap(data)).ToSql()
	return b.conn.ExecCtx(b.ctx, query, vals...)
}

func (b *baseModel[T]) FindRowsByWhere(fields []string, fn func(builder squirrel.SelectBuilder) squirrel.SelectBuilder) ([]*T, error) {
	var resp []*T
	if len(fields) == 0 {
		fields = []string{"*"}
	}
	query, vals, _ := fn(squirrel.Select(fields...).From(b.GetTableNameBySuffix())).ToSql()
	err := b.conn.QueryRowsPartialCtx(b.ctx, &resp, query, vals...)
	return resp, err
}

func (b *baseModel[T]) FindOneByWhere(fields []string, fn func(builder squirrel.SelectBuilder) squirrel.SelectBuilder) (*T, error) {
	var resp T
	if len(fields) == 0 {
		fields = []string{"*"}
	}
	query, vals, _ := fn(squirrel.Select(fields...).From(b.GetTableNameBySuffix())).ToSql()
	err := b.conn.QueryRowPartialCtx(b.ctx, &resp, query, vals...)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func (b *baseModel[T]) GetTableNameBySuffix() string {
	if b.suffix != "" {
		return fmt.Sprint(b.tableName, b.suffix)
	}
	return b.tableName
}

func (b *baseModel[T]) QueryRawRow(data interface{}, query string, values ...interface{}) error {
	err := b.conn.QueryRowPartialCtx(b.ctx, data, query, values...)
	if err != nil {
		return err
	}
	return nil
}

func (b *baseModel[T]) QueryRawRows(data interface{}, query string, values ...interface{}) error {
	return b.conn.QueryRowsPartialCtx(b.ctx, data, query, values...)
}

func (b *baseModel[T]) SetCtx(ctx context.Context) *baseModel[T] {
	b.ctx = ctx
	return b
}
func (b *baseModel[T]) SetConn(conn sqlx.SqlConn) *baseModel[T] {
	b.conn = conn
	return b
}
func (b *baseModel[T]) SetSuffix(s string) *baseModel[T] {
	b.suffix = s
	return b
}
func (b *baseModel[T]) SetAutoIncrKey(s string) *baseModel[T] {
	b.autoIncrKey = s
	return b
}

// TransUpdate model 更新，忽略0值，如果需更新为0值，请使用map方式
func (b *baseModel[T]) TransUpdate(session sqlx.Session, data *T, where func(builder squirrel.UpdateBuilder) squirrel.UpdateBuilder) (sql.Result, error) {
	if where == nil {
		return nil, errors.New("where must be set")
	}
	rafVal := reflect.ValueOf(data).Elem()
	rafTag := reflect.TypeOf(data).Elem()
	var (
		values = make(map[string]interface{}, rafTag.NumField())
	)

	for i := 0; i < rafVal.NumField(); i++ {
		if rafVal.Field(i).IsZero() {
			continue
		}
		tag := rafTag.Field(i).Tag.Get(columnTag)
		if tag == "" {
			tag = rafTag.Field(i).Name
		}
		if tag != b.autoIncrKey {
			values[tag] = rafVal.Field(i).Interface()
		}
	}

	query, vals, _ := where(squirrel.Update(b.GetTableNameBySuffix()).SetMap(values)).ToSql()
	return session.ExecCtx(b.ctx, query, vals...)
}

func (b *baseModel[T]) TransUpdateMap(session sqlx.Session, data map[string]interface{}, where func(builder squirrel.UpdateBuilder) squirrel.UpdateBuilder) (sql.Result, error) {
	query, vals, _ := where(squirrel.Update(b.GetTableNameBySuffix()).SetMap(data)).ToSql()
	return session.ExecCtx(b.ctx, query, vals...)
}

func (b *baseModel[T]) TransUpdateSlice(session sqlx.Session, fields []string, data *T, where func(builder squirrel.UpdateBuilder) squirrel.UpdateBuilder) (sql.Result, error) {
	rafVal := reflect.ValueOf(data).Elem()
	values := make(map[string]interface{}, rafVal.NumField())
	for _, field := range fields {
		values[field] = rafVal.FieldByName(field).Interface()
	}
	query, vals, _ := where(squirrel.Update(b.GetTableNameBySuffix()).SetMap(values)).ToSql()
	return session.ExecCtx(b.ctx, query, vals...)
}

func (b *baseModel[T]) TransInsert(session sqlx.Session, data *T) (sql.Result, error) {
	columns, values := b.buildDataInsert(data)
	query, vals, _ := squirrel.Insert(b.GetTableNameBySuffix()).Columns(columns...).Values(values[0]...).ToSql()
	return session.ExecCtx(b.ctx, query, vals...)
}

func (b *baseModel[T]) TransInsertBatch(session sqlx.Session, data []*T) (sql.Result, error) {
	if len(data) == 0 {
		return nil, nil
	}
	columns, values := b.buildDataInsert(data...)
	var ibuilder = squirrel.Insert(b.GetTableNameBySuffix()).Columns(columns...)
	for i := 0; i < len(values); i++ {
		ibuilder = ibuilder.Values(values[i]...)
	}
	query, vals, _ := ibuilder.ToSql()
	return session.ExecCtx(b.ctx, query, vals...)
}

func (b *baseModel[T]) Count(fn func(builder squirrel.SelectBuilder) squirrel.SelectBuilder) (int, error) {
	var total int
	builder := squirrel.Select("count(*)").From(b.GetTableNameBySuffix())
	if fn != nil {
		builder = fn(builder)
	}
	toSql, values, _ := builder.ToSql()
	err := b.conn.QueryRow(&total, toSql, values...)
	return total, err
}

func (b *baseModel[T]) Exist(fn func(builder squirrel.SelectBuilder) squirrel.SelectBuilder) bool {
	total, _ := b.Count(fn)
	return total > 0
}

type columnExt struct {
	defaultValue string //db 默认值
	structName   string //结构体名
	columnName   string //db 列名
	structType   string //结构体数据类型
	columnType   string //db 数据类型
	nullAble     bool   //是否可null
}

// 构造插入数据
func (b *baseModel[T]) buildDataInsert(in ...*T) ([]string, [][]any) {
	if len(in) == 0 {
		return nil, nil
	}
	var rafTag reflect.Type
	var rafVal reflect.Value

	if reflect.TypeOf(in[0]).Kind() == reflect.Pointer {
		rafTag = reflect.TypeOf(in[0]).Elem()
	} else {
		rafTag = reflect.TypeOf(in[0])
	}
	var columnMap = make(map[string]columnExt, rafTag.NumField())
	var columns = make([]string, 0, rafTag.NumField())
	var values = make([][]any, 0, rafTag.NumField()*len(in))
	for i := 0; i < rafTag.NumField(); i++ {
		tags := rafTag.Field(i).Tag
		columnName := tags.Get(columnTag)
		if columnName == b.autoIncrKey || columnName == "" {
			continue
		}
		columns = append(columns, columnName)
		columnMap[columnName] = columnExt{
			defaultValue: tags.Get(defaultValue),
			structName:   rafTag.Field(i).Name,
			columnName:   columnName,
			structType:   rafTag.Field(i).Type.String(),
			columnType:   tags.Get(columnType),
			nullAble:     tags.Get(isNullAble) == nullAble,
		}
	}

	for i := 0; i < len(in); i++ {
		if reflect.ValueOf(in[i]).Kind() == reflect.Pointer {
			rafVal = reflect.ValueOf(in[i]).Elem()
		} else {
			rafVal = reflect.ValueOf(in[i])
		}
		var value = make([]any, len(columns))
		for k := 0; k < len(columns); k++ {
			cExt := columnMap[columns[k]]
			switch cExt.structType {
			case "driver.Value":
				if cExt.defaultValue != "" && rafVal.FieldByName(cExt.structName).IsNil() {
					if strings.Contains(cExt.columnType, "bit") {
						value[k] = squirrel.Expr(cExt.defaultValue)
					} else {
						value[k] = cExt.defaultValue
					}
				} else {
					value[k] = rafVal.FieldByName(cExt.structName).Interface()
				}
			default:
				if rafVal.FieldByName(cExt.structName).IsZero() {
					if cExt.defaultValue != "" {
						switch cExt.structType {
						case "string", "sql.NullString", "text", "crmx.NullString":
							value[k] = cExt.defaultValue
						case "time.Time":
							//第一个字符是数字
							if cExt.defaultValue[0] >= 48 && cExt.defaultValue[0] <= 57 {
								value[k] = cExt.defaultValue
							} else {
								value[k] = squirrel.Expr(cExt.defaultValue)
							}
						default:
							value[k] = squirrel.Expr(cExt.defaultValue)
						}
					} else {
						if cExt.nullAble {
							value[k] = squirrel.Expr("NULL")
						} else {
							value[k] = rafVal.FieldByName(cExt.structName).Interface()
						}
					}
				} else {
					value[k] = rafVal.FieldByName(cExt.structName).Interface()
				}
			}
		}
		values = append(values, value)
	}

	return columns, values
}

type NullString sql.NullString

func (rec *NullString) Scan(value interface{}) error {
	var x sql.NullString
	err := x.Scan(value)
	if err != nil {
		return err
	}
	*rec = NullString(x)
	return nil
}
func (rec NullString) Value() (driver.Value, error) {
	return sql.NullString(rec).Value()
}
func (rec NullString) MarshalJSON() ([]byte, error) {
	if rec.Valid {
		return json.Marshal(rec.String)
	}
	return jsonNull, nil
}
func (rec *NullString) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, jsonNull) {
		rec.Valid = false
		return nil
	}
	rec.Valid = true
	return json.Unmarshal(data, &rec.String)
}

type NullInt64 sql.NullInt64

func (rec *NullInt64) Scan(value interface{}) error {
	var x sql.NullInt64
	err := x.Scan(value)
	if err != nil {
		return err
	}
	*rec = NullInt64(x)
	return nil
}
func (rec NullInt64) Value() (driver.Value, error) {
	return sql.NullInt64(rec).Value()
}

func (rec NullInt64) MarshalJSON() ([]byte, error) {
	if rec.Valid {
		return json.Marshal(rec.Int64)
	}
	return jsonNull, nil
}

func (rec *NullInt64) UnmarshalJSON(data []byte) error {
	if !bytes.Equal(data, jsonNull) {
		if err := json.Unmarshal(data, &rec.Int64); err != nil {
			return err
		}
		rec.Valid = true
	}
	return nil
}

type NullInt16 sql.NullInt16

func (rec *NullInt16) Scan(value interface{}) error {
	var x sql.NullInt16
	err := x.Scan(value)
	if err != nil {
		return err
	}
	*rec = NullInt16(x)
	return nil
}
func (rec NullInt16) Value() (driver.Value, error) {
	return sql.NullInt16(rec).Value()
}

func (rec NullInt16) MarshalJSON() ([]byte, error) {
	if rec.Valid {
		return json.Marshal(rec.Int16)
	}
	return jsonNull, nil
}

func (rec *NullInt16) UnmarshalJSON(data []byte) error {
	if !bytes.Equal(data, jsonNull) {
		if err := json.Unmarshal(data, &rec.Int16); err != nil {
			return err
		}
		rec.Valid = true
	}
	return nil
}

type NullInt32 sql.NullInt32

func (rec *NullInt32) Scan(value interface{}) error {
	var x sql.NullInt32
	err := x.Scan(value)
	if err != nil {
		return err
	}
	*rec = NullInt32(x)
	return nil
}
func (rec NullInt32) Value() (driver.Value, error) {
	return sql.NullInt32(rec).Value()
}

func (rec NullInt32) MarshalJSON() ([]byte, error) {
	if rec.Valid {
		return json.Marshal(rec.Int32)
	}
	return jsonNull, nil
}

func (rec *NullInt32) UnmarshalJSON(data []byte) error {
	if !bytes.Equal(data, jsonNull) {
		if err := json.Unmarshal(data, &rec.Int32); err != nil {
			return err
		}
		rec.Valid = true
	}
	return nil
}

type NullTime sql.NullTime

func (rec *NullTime) Scan(value interface{}) error {
	var x sql.NullTime
	err := x.Scan(value)
	if err != nil {
		return err
	}
	*rec = NullTime(x)
	return nil
}
func (rec NullTime) Value() (driver.Value, error) {
	return sql.NullTime(rec).Value()
}

func (rec NullTime) MarshalJSON() ([]byte, error) {
	if rec.Valid {
		return rec.Time.MarshalJSON()
	}
	return jsonNull, nil
}
func (rec *NullTime) UnmarshalJSON(data []byte) error {
	if !bytes.Equal(data, jsonNull) {
		if err := json.Unmarshal(data, &rec.Time); err != nil {
			return err
		}
		rec.Valid = true
	}
	return nil
}

type NullBool sql.NullBool

func (rec *NullBool) Scan(value interface{}) error {
	var x sql.NullBool
	err := x.Scan(value)
	if err != nil {
		return err
	}
	*rec = NullBool(x)
	return nil
}
func (rec NullBool) Value() (driver.Value, error) {
	return sql.NullBool(rec).Value()
}

func (rec NullBool) MarshalJSON() ([]byte, error) {
	if rec.Valid {
		return json.Marshal(rec.Bool)
	}
	return jsonNull, nil
}

func (rec *NullBool) UnmarshalJSON(data []byte) error {
	if !bytes.Equal(data, jsonNull) {
		if err := json.Unmarshal(data, &rec.Bool); err != nil {
			return err
		}
		rec.Valid = true
	}
	return nil
}

type NullByte sql.NullByte

func (rec *NullByte) Scan(value interface{}) error {
	var x sql.NullByte
	err := x.Scan(value)
	if err != nil {
		return err
	}
	*rec = NullByte(x)
	return nil
}
func (rec NullByte) Value() (driver.Value, error) {
	return sql.NullByte(rec).Value()
}

func (rec *NullByte) MarshalJSON() ([]byte, error) {
	if rec.Valid {
		return json.Marshal(rec.Byte)
	}
	return jsonNull, nil
}

func (rec *NullByte) UnmarshalJSON(data []byte) error {
	if !bytes.Equal(data, jsonNull) {
		if err := json.Unmarshal(data, &rec.Byte); err != nil {
			return err
		}
		rec.Valid = true
	}
	return nil
}

type NullFloat64 sql.NullFloat64

func (rec *NullFloat64) Scan(value interface{}) error {
	var x sql.NullFloat64
	err := x.Scan(value)
	if err != nil {
		return err
	}
	*rec = NullFloat64(x)
	return nil
}
func (rec NullFloat64) Value() (driver.Value, error) {
	return sql.NullFloat64(rec).Value()
}

func (rec NullFloat64) MarshalJSON() ([]byte, error) {
	if rec.Valid {
		return json.Marshal(rec.Float64)
	}
	return jsonNull, nil
}

func (rec *NullFloat64) UnmarshalJSON(data []byte) error {
	if !bytes.Equal(data, jsonNull) {
		if err := json.Unmarshal(data, &rec.Float64); err != nil {
			return err
		}
		rec.Valid = true
	}
	return nil
}

type NullDecimal decimal.NullDecimal

func (d *NullDecimal) Scan(value interface{}) error {
	var x decimal.NullDecimal
	err := x.Scan(value)
	if err != nil {
		return err
	}
	*d = NullDecimal(x)
	return nil
}
func (d NullDecimal) Value() (driver.Value, error) {
	return decimal.NullDecimal(d).Value()
}

func (d NullDecimal) MarshalJSON() ([]byte, error) {
	if !d.Valid {
		return jsonNull, nil
	}
	return d.Decimal.MarshalJSON()
}
func (d *NullDecimal) UnmarshalJSON(data []byte) error {
	err := d.Decimal.UnmarshalJSON(data)
	if err != nil {
		return err
	}
	d.Valid = true
	return nil
}

func CreateNullBool(b bool) NullBool {
	return NullBool{
		Bool:  b,
		Valid: true,
	}
}

func CreateNullString(s string) NullString {
	return NullString{
		String: s,
		Valid:  true,
	}
}

func CreateNullTime(t time.Time) NullTime {
	return NullTime{
		Time:  t,
		Valid: !t.IsZero(),
	}
}

func CreateNullInt64(i int64) NullInt64 {
	return NullInt64{
		Int64: int64(i),
		Valid: true,
	}
}

func CreateNullFloat64(f float64) NullFloat64 {
	return NullFloat64{
		Float64: f,
		Valid:   true,
	}
}

func CreateNullDecimal(f decimal.Decimal) NullDecimal {
	return NullDecimal{
		Decimal: f,
		Valid:   true,
	}
}

func (r *Repository) GetConn() sqlx.SqlConn {
	return r.conn
}

func IsEnabled(x any) bool {
	if by, ok := x.([]byte); ok {
		if len(by) > 0 && by[0] == 1 {
			return true
		}
	}
	return btype.IsBool(x)
}

func (rec NullTime) ToString(layout string) string {
	if rec.Valid {
		if layout == "" {
			return rec.Time.String()
		}
		return rec.Time.Format(layout)
	}
	return ""
}
