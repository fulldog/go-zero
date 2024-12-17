package gen

import (
	"github.com/zeromicro/go-zero/tools/goctl/model/sql/template"
	"github.com/zeromicro/go-zero/tools/goctl/util"
	"github.com/zeromicro/go-zero/tools/goctl/util/pathx"
)

func genTag(table Table, in string) (string, error) {
	if in == "" {
		return in, nil
	}

	text, err := pathx.LoadTemplate(category, tagTemplateFile, template.Tag)
	if err != nil {
		return "", err
	}
	var def, columnType, isNullAble, isAutoIncrKey string
	for _, field := range table.Fields {
		if field.Name.Source() == in {
			def = field.DefaultValue
			columnType = field.ColumnType
			isNullAble = field.IsNullAble
			if field.IsAutoIncr {
				isAutoIncrKey = "1"
			}
			break
		}
	}
	output, err := util.With("tag").Parse(text).Execute(map[string]any{
		"field":         in,
		"defaultValue":  def,
		"columnType":    columnType,
		"data":          table,
		"isNullAble":    isNullAble,
		"isAutoIncrKey": isAutoIncrKey,
	})
	if err != nil {
		return "", err
	}

	return output.String(), nil
}
