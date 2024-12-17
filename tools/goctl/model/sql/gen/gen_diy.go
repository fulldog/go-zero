package gen

import (
	"github.com/zeromicro/go-zero/tools/goctl/model/sql/parser"
	"github.com/zeromicro/go-zero/tools/goctl/model/sql/template"
	"github.com/zeromicro/go-zero/tools/goctl/util"
	"github.com/zeromicro/go-zero/tools/goctl/util/pathx"
	"github.com/zeromicro/go-zero/tools/goctl/util/stringx"
	"os"
	"path/filepath"
)

func genDiy(g *defaultGenerator) error {
	text, err := pathx.LoadTemplate(category, diyTemplateFile, template.Diy)
	if err != nil {
		return err
	}
	output, err := util.With("diy").Parse(text).
		GoFmt(true).Execute(map[string]any{
		"pkg": g.pkg,
	})
	if err != nil {
		return err
	}
	//_ = os.Mkdir(filepath.Join(g.dir, "diy"), os.ModePerm)
	err = os.WriteFile(filepath.Join(g.dir, "base_model.go"), output.Bytes(), os.ModePerm)
	return err
}

func genDiyModel(g *defaultGenerator, in parser.Table) error {
	text, err := pathx.LoadTemplate(category, diyModelTemplateFile, template.DiyModel)
	if err != nil {
		return err
	}

	t := util.With("diyModel").
		Parse(text).
		GoFmt(true)
	output, err := t.Execute(map[string]any{
		"pkg":                   g.pkg,
		"upperStartCamelObject": in.Name.ToCamel(),
		"lowerStartCamelObject": stringx.From(in.Name.ToCamel()).Untitle(),
	})
	if err != nil {
		return err
	}
	err = os.WriteFile(filepath.Join(g.dir, "diyModel.go"), output.Bytes(), os.ModePerm)
	return err

}
