package gogen

import (
	_ "embed"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/zeromicro/go-zero/tools/goctl/api/spec"
	apiutil "github.com/zeromicro/go-zero/tools/goctl/api/util"
	"github.com/zeromicro/go-zero/tools/goctl/config"
	"github.com/zeromicro/go-zero/tools/goctl/util"
	"github.com/zeromicro/go-zero/tools/goctl/util/format"
)

const typesFile = "types"

//go:embed types.tpl
var typesTemplate string

// BuildTypes gen types to string
func BuildTypes(types []spec.Type) (string, error) {
	var builder strings.Builder
	first := true
	for _, tp := range types {
		if first {
			first = false
		} else {
			builder.WriteString("\n\n")
		}
		if err := writeType(&builder, tp); err != nil {
			return "", apiutil.WrapErr(err, "Type "+tp.Name()+" generate error")
		}
	}

	return builder.String(), nil
}

func genTypes(dir string, cfg *config.Config, api *spec.ApiSpec) error {
	val, err := BuildTypes(api.Types)
	if err != nil {
		return err
	}

	typeFilename, err := format.FileNamingFormat(cfg.NamingFormat, typesFile)
	if err != nil {
		return err
	}

	typeFilename = typeFilename + ".go"
	filename := path.Join(dir, typesDir, typeFilename)
	os.Remove(filename)

	return genFile(fileGenConfig{
		dir:             dir,
		subdir:          typesDir,
		filename:        typeFilename,
		templateName:    "typesTemplate",
		category:        category,
		templateFile:    typesTemplateFile,
		builtinTemplate: typesTemplate,
		data: map[string]any{
			"types":        val,
			"containsTime": false,
		},
	})
}

//go:embed diy.tpl
var diyTemplate string

//go:embed diyJson.tpl
var diyJsonTemplate string

func genDiy(dir string, cfg *config.Config, api *spec.ApiSpec) error {
	typeFilename, err := format.FileNamingFormat(cfg.NamingFormat, "diy")

	if err != nil {
		return err
	}

	typeFilename = typeFilename + ".go"
	filename := path.Join(dir, typesDir, typeFilename)
	os.Remove(filename)
	metaBuilder := strings.Builder{}
	for _, g := range api.Service.Groups {
		for _, r := range g.Routes {
			routeComment := r.JoinedDoc()
			if len(routeComment) == 0 {
				routeComment = "N/A"
			}
			//lastpath := strings.Split(r.Path, "/")
			req := strings.Title(r.RequestTypeName())
			res := strings.Title(r.ResponseTypeName())
			if req == "" {
				req = "Null"
			}
			if res == "" {
				res = "Null"
			}
			metaBuilder.WriteString(fmt.Sprintf(`"%s":{false,%s,"%s","%s",metadataItem{&%s{},"%s"},metadataItem{&%s{},"%s"}},`, g.Annotation.Properties["prefix"]+r.Path, routeComment, g.Annotation.Properties["prefix"]+r.Path, r.Method, req, "-", res, "-"))
			metaBuilder.WriteString("\r\n")
		}
	}

	return genFile(fileGenConfig{
		dir:             dir,
		subdir:          typesDir,
		filename:        typeFilename,
		templateName:    "typesTemplate",
		category:        category,
		templateFile:    "diy.tpl",
		builtinTemplate: diyTemplate,
		data: map[string]any{
			"metadata": metaBuilder.String(),
		},
	})
}
func genDiyJson(dir string, cfg *config.Config, api *spec.ApiSpec) error {
	var diyJson = `
//MarshalJSON %s
func (js %s) MarshalJSON() ([]byte, error) {
	type Alias %s
	alias := Alias(js)
	return jsoniter.ConfigCompatibleWithStandardLibrary.Marshal(alias)
}`
	var diyJsonBuilder = strings.Builder{}
	for _, t := range api.Types {
		diyJsonBuilder.WriteString(fmt.Sprintf(diyJson, t.Name(), t.Name(), t.Name()))
	}
	return genFile(fileGenConfig{
		dir:             dir,
		subdir:          typesDir,
		filename:        "diyJson.go",
		templateName:    "typesTemplate",
		category:        category,
		templateFile:    "diyJson.tpl",
		builtinTemplate: diyJsonTemplate,
		data: map[string]any{
			"diyJson": diyJsonBuilder.String(),
		},
	})
}

func writeType(writer io.Writer, tp spec.Type) error {
	structType, ok := tp.(spec.DefineStruct)
	if !ok {
		return fmt.Errorf("unspport struct type: %s", tp.Name())
	}

	fmt.Fprintf(writer, "type %s struct {\n", util.Title(tp.Name()))
	for _, member := range structType.Members {
		if member.IsInline {
			if _, err := fmt.Fprintf(writer, "%s\n", strings.Title(member.Type.Name())); err != nil {
				return err
			}

			continue
		}

		if err := writeProperty(writer, member.Name, member.Tag, member.GetComment(), member.Type, 1); err != nil {
			return err
		}
	}
	fmt.Fprintf(writer, "}")
	return nil
}
