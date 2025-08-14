package admin

import "net/http"

func serveCSS(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/css")
	w.Write([]byte(`body{font-family:system-ui,Segoe UI,Roboto,Arial,sans-serif;margin:0;background:#0b0c0f;color:#e6e6e6}
a{color:#91c9ff;text-decoration:none} a:hover{text-decoration:underline}
header{padding:12px 20px;border-bottom:1px solid #1b1d22;background:#111318}
.container{max-width:1100px;margin:0 auto;padding:20px}
table{width:100%;border-collapse:collapse;border:1px solid #2a2d34}
th,td{padding:10px;border-bottom:1px solid #2a2d34} th{text-align:left;background:#151720}
.btn{display:inline-block;padding:8px 12px;border:1px solid #2a2d34;background:#1a1d26;color:#e6e6e6;border-radius:6px}
.btn-primary{background:#2563eb;border-color:#2563eb} .btn-danger{background:#b91c1c;border-color:#b91c1c}
input,textarea,select{width:100%;padding:8px;background:#0f1116;color:#e6e6e6;border:1px solid #2a2d34;border-radius:6px}
.grid{display:grid;gap:16px} .cols-2{grid-template-columns:1fr 1fr}
.card{border:1px solid #2a2d34;border-radius:10px;padding:16px;background:#0f1116}
h1,h2,h3{margin:12px 0}
.small{opacity:.7} .mono{font-family:ui-monospace,Menlo,Consolas,monospace}
code,pre{background:#0f1116;border:1px solid #2a2d34;border-radius:8px;padding:8px;display:block;white-space:pre-wrap}`))
}

func serveJS(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/javascript")
	w.Write([]byte(`async function postJSON(url, body){const r=await fetch(url,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body||{})});return r.json()}
async function postForm(url, form){const r=await fetch(url,{method:'POST',body:form});if(r.headers.get('content-type')?.includes('application/json'))return r.json();return r.text()}
function toast(t){alert(t)}
`))
}
