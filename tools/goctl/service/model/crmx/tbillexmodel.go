package crmx

import "github.com/zeromicro/go-zero/core/stores/sqlx"

var _ TBillExModel = (*customTBillExModel)(nil)

type (
	// TBillExModel is an interface to be customized, add more methods here,
	// and implement the added methods in customTBillExModel.
	TBillExModel interface {
		tBillExModel
		withSession(session sqlx.Session) TBillExModel
	}

	customTBillExModel struct {
		*defaultTBillExModel
	}
)

// NewTBillExModel returns a model for the database table.
func NewTBillExModel(conn sqlx.SqlConn) TBillExModel {
	return &customTBillExModel{
		defaultTBillExModel: newTBillExModel(conn),
	}
}

func (m *customTBillExModel) withSession(session sqlx.Session) TBillExModel {
	return NewTBillExModel(sqlx.NewSqlConnFromSession(session))
}
