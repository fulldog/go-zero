package {{.pkg}}

import (
	"context"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// DiyNew{{.upperStartCamelObject}}Model returns a model for the database table.
func DiyNew{{.upperStartCamelObject}}Model(conn sqlx.SqlConn) {{.upperStartCamelObject}}Model {
	m := &custom{{.upperStartCamelObject}}Model{
		default{{.upperStartCamelObject}}Model: new{{.upperStartCamelObject}}Model(conn),
	}
	m.baseModel = &baseModel[{{.upperStartCamelObject}}]{
    		conn:        conn,
    		ctx:         context.Background(),
    		tableName:   m.table,
    		autoIncrKey: {{.lowerStartCamelObject}}AutoIncrKey,
    	}
	return m
}
