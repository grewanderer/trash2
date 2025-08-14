package netjson

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"text/template"
)

// Source — единица слияния NetJSON с приоритетом.
type Source struct {
	Name     string
	Priority int            // больший приоритет выигрывает
	JSON     map[string]any // разобранный NetJSON
}

// Merge объединяет несколько NetJSON-источников по приоритету.
// При конфликте ключей: побеждает источник с БОЛЬШИМ Priority.
// Для map — глубокое слияние; для slice и скаляров — замена целиком.
func Merge(sources ...Source) (map[string]any, error) {
	if len(sources) == 0 {
		return map[string]any{}, nil
	}
	// сортируем по возрастанию приоритета и накатываем по очереди
	sort.SliceStable(sources, func(i, j int) bool { return sources[i].Priority < sources[j].Priority })
	var out map[string]any
	for i, s := range sources {
		if s.JSON == nil {
			continue
		}
		if i == 0 {
			out = cloneMap(s.JSON)
			continue
		}
		out = deepMerge(out, s.JSON)
	}
	return out, nil
}

func deepMerge(dst, src map[string]any) map[string]any {
	if dst == nil {
		return cloneMap(src)
	}
	out := cloneMap(dst)
	for k, v := range src {
		if dv, ok := out[k]; ok {
			// map + map → merge
			if dm, ok1 := dv.(map[string]any); ok1 {
				if sm, ok2 := toMap(v); ok2 {
					out[k] = deepMerge(dm, sm)
					continue
				}
			}
		}
		// иначе — просто заменить
		out[k] = cloneValue(v)
	}
	return out
}

func toMap(v any) (map[string]any, bool) {
	switch vv := v.(type) {
	case map[string]any:
		return vv, true
	default:
		return nil, false
	}
}

func cloneMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	b, _ := json.Marshal(m)
	var out map[string]any
	_ = json.Unmarshal(b, &out)
	return out
}

func cloneValue(v any) any {
	b, _ := json.Marshal(v)
	var out any
	_ = json.Unmarshal(b, &out)
	return out
}

// ApplyVars применяет переменные к NetJSON.
// 1) Текстовые значения обрабатываются как Go template с данными из vars.
// 2) Объект {"$var":"a.b.c","default":<val>} заменяется значением из vars по пути.
func ApplyVars(nj map[string]any, vars map[string]any) (map[string]any, error) {
	res := cloneMap(nj)
	var err error
	res, err = applyVarObjects(res, vars)
	if err != nil {
		return nil, err
	}
	res, err = applyStringTemplates(res, vars)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func applyVarObjects(v any, vars map[string]any) (map[string]any, error) {
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("applyVarObjects expects map")
	}

	var walk func(x any) any // ← предварительное объявление

	walk = func(x any) any {
		switch t := x.(type) {
		case map[string]any:
			if path, ok := t["$var"].(string); ok {
				val, ok := lookup(vars, path)
				if !ok {
					if def, has := t["default"]; has {
						return def
					}
					return x // оставляем как есть
				}
				return val
			}
			y := make(map[string]any, len(t))
			for k, vv := range t {
				y[k] = walk(vv)
			}
			return y
		case []any:
			y := make([]any, len(t))
			for i := range t {
				y[i] = walk(t[i])
			}
			return y
		default:
			return x
		}
	}

	anyRes := walk(m)
	out, _ := anyRes.(map[string]any)
	return out, nil
}

func applyStringTemplates(v any, data map[string]any) (map[string]any, error) {
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("applyStringTemplates expects map")
	}
	funcs := template.FuncMap{
		"upper": strings.ToUpper,
		"lower": strings.ToLower,
		"default": func(def any, val any) any {
			if isZero(val) {
				return def
			}
			return val
		},
		"join": func(a []any, sep string) string {
			parts := make([]string, 0, len(a))
			for _, it := range a {
				parts = append(parts, fmt.Sprint(it))
			}
			return strings.Join(parts, sep)
		},
	}
	var render func(x any) any
	render = func(x any) any {
		switch t := x.(type) {
		case string:
			if !strings.Contains(t, "{{") {
				return t
			}
			tpl, err := template.New("v").Funcs(funcs).Option("missingkey=zero").Parse(t)
			if err != nil {
				return t
			}
			var buf bytes.Buffer
			if err := tpl.Execute(&buf, data); err != nil {
				return t
			}
			return buf.String()
		case map[string]any:
			y := make(map[string]any, len(t))
			for k, vv := range t {
				y[k] = render(vv)
			}
			return y
		case []any:
			y := make([]any, len(t))
			for i := range t {
				y[i] = render(t[i])
			}
			return y
		default:
			return t
		}
	}
	anyRes := render(m)
	out, _ := anyRes.(map[string]any)
	return out, nil
}

func isZero(v any) bool { return v == nil || v == "" || v == 0 || v == false }

// lookup читает значение из vars по пути вида "a.b.c".
func lookup(vars map[string]any, path string) (any, bool) {
	cur := any(vars)
	for _, p := range strings.Split(path, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = m[p]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}
