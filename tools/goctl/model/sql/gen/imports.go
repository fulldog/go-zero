package gen

import (
	"fmt"
	"github.com/zeromicro/go-zero/tools/goctl/model/sql/template"
	"github.com/zeromicro/go-zero/tools/goctl/util"
	"github.com/zeromicro/go-zero/tools/goctl/util/pathx"
	"strings"
)

func genImports(table Table, withCache, timeImport bool) (string, error) {
	var thirdImports []string
	var m = map[string]struct{}{}
	for _, c := range table.Fields {
		if len(c.ThirdPkg) > 0 {
			if _, ok := m[c.ThirdPkg]; ok {
				continue
			}
			m[c.ThirdPkg] = struct{}{}
			thirdImports = append(thirdImports, fmt.Sprintf("%q", c.ThirdPkg))
		}
	}

	if withCache {
		text, err := pathx.LoadTemplate(category, importsTemplateFile, template.Imports)
		if err != nil {
			return "", err
		}

		buffer, err := util.With("import").Parse(text).Execute(map[string]any{
			"time":       timeImport,
			"containsPQ": table.ContainsPQ,
			"data":       table,
			"third":      strings.Join(thirdImports, "\n"),
		})
		if err != nil {
			return "", err
		}

		return buffer.String(), nil
	}

	text, err := pathx.LoadTemplate(category, importsWithNoCacheTemplateFile, template.ImportsNoCache)
	if err != nil {
		return "", err
	}
	var driverAny = ""
	var uuid = ""
	var decimal = ""
	for _, field := range table.Table.Fields {
		if driverAny == "" && field.DataType == "driver.Value" {
			driverAny = "database/sql/driver"
		}
		if uuid == "" && (strings.Contains(strings.ToLower(field.Name.Source()), "uuid") || strings.ToLower(field.ColumnType) == "char(36)") {
			//uuid = "github.com/google/uuid"
		}
		if decimal == "" && strings.Contains(field.DataType, "decimal") {
			decimal = "decimal.Decimal"
		}
	}
	buffer, err := util.With("import").Parse(text).Execute(map[string]any{
		"time":       timeImport,
		"containsPQ": table.ContainsPQ,
		"data":       table,
		"third":      strings.Join(thirdImports, "\n"),
		"driverAny":  driverAny,
		"uuid":       uuid,
		"decimal":    decimal,
	})
	if err != nil {
		return "", err
	}

	return buffer.String(), nil
}
