package admin

import (
	"embed"
	"html/template"
	"io/fs"
	"log"
	"path"
)

//go:embed templates/*.tmpl
var tplFS embed.FS

// набор готовых шаблонов по страницам (ключ = имя файла страницы, напр. "devices_list.tmpl")
type pageTemplates map[string]*template.Template

func parseTemplates() pageTemplates {
	// найдём все .tmpl
	all, err := fs.Glob(tplFS, "templates/*.tmpl")
	if err != nil {
		log.Fatalf("admin: glob templates failed: %v", err)
	}
	if len(all) == 0 {
		log.Fatalf("admin: no templates found in embed FS")
	}

	// соберём по одной паре: layout + конкретная страница
	out := make(pageTemplates)
	for _, f := range all {
		if path.Base(f) == "layout.tmpl" {
			continue
		}
		// для каждой страницы создаём отдельный набор
		t := template.New("layout")
		if _, err := t.ParseFS(tplFS, "templates/layout.tmpl"); err != nil {
			log.Fatalf("admin: parse layout.tmpl: %v", err)
		}
		if _, err := t.ParseFS(tplFS, f); err != nil {
			log.Fatalf("admin: parse %s: %v", f, err)
		}
		key := path.Base(f) // например, "devices_list.tmpl"
		out[key] = t
	}
	return out
}
