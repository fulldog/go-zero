package types
//重写JSON序列化以实现某些字段的字面量翻译，只生成骨架代码，如不需要请创建空文档
import (
	jsoniter "github.com/json-iterator/go"
)
{{.diyJson}}
