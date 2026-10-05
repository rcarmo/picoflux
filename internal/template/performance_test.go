package template

import (
	"html/template"
	"testing"
)

func benchmarkEngine() *Engine {
	e := NewEngine("")
	e.templates["sample"] = template.Must(template.New("base").Funcs(template.FuncMap{"t": func(string, ...any) string { return "" }, "plural": func(string, int, ...any) string { return "" }}).Parse(`{{t "menu.unread"}}{{range .items}}<article>{{.}}</article>{{end}}`))
	return e
}
func BenchmarkRender(b *testing.B) {
	e := benchmarkEngine()
	data := map[string]any{"language": "en_US", "items": make([]string, 100)}
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			e.Render("sample", data)
		}
	})
}
