// internal/admin/templates.go
package admin

import (
	"embed"
	"html/template"
	"io/fs"
	"log"
	"strconv"
	"time"
)

//go:embed templates/*.tmpl
var tplFS embed.FS

func parseTemplates() *template.Template {
	t := template.New("").Funcs(template.FuncMap{
		"since": since, // ← регистрируем хелпер
	})

	list, err := fs.Glob(tplFS, "templates/*.tmpl")
	if err != nil || len(list) == 0 {
		log.Printf("admin: no templates matched in embed FS (err=%v)", err)
		min, _ := t.Parse(`{{define "layout"}}<html><body>{{block "content" .}}{{end}}</body></html>{{end}}`)
		return min
	}

	ts, err := t.ParseFS(tplFS, list...)
	if err != nil {
		log.Printf("admin: template parse error: %v", err)
		min, _ := t.Parse(`{{define "layout"}}<html><body>{{block "content" .}}{{end}}</body></html>{{end}}`)
		return min
	}
	return ts
}

// since возвращает короткое "n ago" для time.Time или *time.Time.
// Если значение пустое — возвращает "-".
func since(v any) string {
	if v == nil {
		return "-"
	}
	var t time.Time
	switch x := v.(type) {
	case time.Time:
		t = x
	case *time.Time:
		if x == nil {
			return "-"
		}
		t = *x
	default:
		return "-"
	}
	if t.IsZero() {
		return "-"
	}
	d := time.Since(t)
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		return plural(int(d.Minutes()), "min") + " ago"
	}
	if d < 24*time.Hour {
		return plural(int(d.Hours()), "h") + " ago"
	}
	days := int(d.Hours() / 24)
	return plural(days, "day") + " ago"
}

func plural(n int, unit string) string {
	return fmtInt(n) + " " + unit
}

func fmtInt(n int) string {
	// без локали, коротко
	return strconv.Itoa(n)
}
