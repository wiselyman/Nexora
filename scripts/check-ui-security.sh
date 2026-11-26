#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
html="$root/internal/ui/index.html"
privacy="$root/internal/ui/privacy.html"
js="$root/internal/ui/assets/app.js"
approval_js="$root/internal/ui/assets/approval-text.js"
css="$root/internal/ui/assets/app.css"

node --check "$js"
node --check "$approval_js"
node "$root/scripts/test-approval-text.js"
test -f "$css"
test -f "$privacy"
rg -q --fixed-strings 'href="/assets/app.css"' "$html"
rg -q --fixed-strings 'src="/assets/app.js"' "$html"
rg -q --fixed-strings 'src="/assets/approval-text.js"' "$html"
rg -q --fixed-strings 'href="/assets/app.css"' "$privacy"

if rg -i -n --pcre2 '<script(?![^>]*\bsrc=)[^>]*>|<style(?:\s|>)|\sstyle\s*=|\son[a-z][a-z0-9_-]*\s*=' "$html" "$privacy"; then
  printf 'Inline UI script/style or event attributes are blocked by the CSP; move behavior and styles to same-origin assets.\n' >&2
  exit 1
fi

if rg -n 'innerHTML|outerHTML|insertAdjacentHTML|document\.write|eval\(|new Function' "$root/internal/ui"; then
  printf 'Unsafe HTML or dynamic-code sink found in the UI.\n' >&2
  exit 1
fi

for directive in \
  "default-src 'none'" \
  "base-uri 'none'" \
  "object-src 'none'" \
  "script-src 'self'" \
  "style-src 'self'" \
  "img-src 'self'" \
  "connect-src 'self'" \
  "form-action 'none'" \
  "frame-ancestors 'none'"; do
  if ! rg -q --fixed-strings "$directive" "$root/internal/app/http.go"; then
    printf 'Required Content-Security-Policy directive is missing: %s\n' "$directive" >&2
    exit 1
  fi
done
rg -q --fixed-strings '"/assets/app.js"' "$root/internal/app/http.go"
rg -q --fixed-strings '"/assets/approval-text.js"' "$root/internal/app/http.go"
rg -q --fixed-strings '"/assets/app.css"' "$root/internal/app/http.go"
rg -q --fixed-strings '"/privacy"' "$root/internal/app/http.go"
rg -q --fixed-strings '"/api/runtime-key/clear", h.requireUIAuth(http.HandlerFunc(h.clearRuntimeAPIKey))' "$root/internal/app/http.go"
rg -U -q 'func \(h \*HTTP\) clearRuntimeAPIKey\(w http.ResponseWriter, r \*http.Request\) \{\s*if r.Method != http.MethodPost \|\| !sameOrigin\(r\)' "$root/internal/app/http.go"
rg -q --fixed-strings 'w.Header().Set("Cache-Control", "no-store")' "$root/internal/app/http.go"
rg -q --fixed-strings '//go:embed index.html privacy.html assets/*' "$root/internal/ui/ui.go"
rg -q --fixed-strings 'if(savePromise)return savePromise' "$js"
rg -q --fixed-strings "const apiKeyValue=\$('apiKey').value" "$js"
rg -q --fixed-strings "if(formRevision!==savedRevision){toast('保存期间配置又有修改；请先保存更改，再启动 Tunnel');return}" "$js"
rg -q --fixed-strings "window.addEventListener('beforeunload'" "$js"
rg -q --fixed-strings 'displayApprovalText(item.command)' "$js"
rg -q --fixed-strings '拒绝命令请求 ${requestRef}' "$js"
rg -q --fixed-strings '仅允许命令请求 ${requestRef} 一次' "$js"

printf 'UI security policy checks passed.\n'
