// tray の試作の一覧の面。Access の内側に置き、ホーム画面から開いて Access のログインを通れるかを見る。
// /pwa/ の下は静的なファイルで、Worker のコードを通らずに返る。
export default {
  async fetch(request, env) {
    // Access が前にいれば、通った要求には必ずこのヘッダが付く。無ければ Access を通っていない。
    if (!request.headers.get('cf-access-jwt-assertion')) {
      return new Response('Access を通っていない要求は断る。\n', { status: 403, headers: { 'content-type': 'text/plain; charset=utf-8' } });
    }
    if (!env.VIEW_ORIGIN) {
      return new Response('VIEW_ORIGIN が設定にない。\n', { status: 500, headers: { 'content-type': 'text/plain; charset=utf-8' } });
    }
    const url = new URL(request.url);
    if (url.pathname === '/api/check') {
      return Response.json({ access: true, at: new Date().toISOString() }, { headers: { 'cache-control': 'no-store' } });
    }
    if (url.pathname !== '/') {
      return new Response('無い。\n', { status: 404, headers: { 'content-type': 'text/plain; charset=utf-8' } });
    }
    return new Response(page(env.VIEW_ORIGIN), {
      headers: {
        'content-type': 'text/html; charset=utf-8',
        'content-security-policy': "frame-ancestors 'none'",
        'cache-control': 'no-store',
      },
    });
  },
};

function page(view) {
  const v = escape(view);
  return `<!doctype html>
<html lang="ja">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover">
<title>tray（試作）</title>
<link rel="manifest" href="/pwa/manifest.webmanifest">
<link rel="apple-touch-icon" href="/pwa/apple-touch-icon.png">
<meta name="apple-mobile-web-app-capable" content="yes">
<meta name="apple-mobile-web-app-title" content="tray">
<meta name="theme-color" content="#1f2937">
<style>
  body { font-family: system-ui, sans-serif; margin: 0; padding: env(safe-area-inset-top) 12px 12px; }
  dt { font-weight: bold; margin-top: 8px; }
  iframe { width: 100%; height: 50vh; border: 1px solid #999; }
  a { display: inline-block; margin: 6px 0; }
</style>
</head>
<body>
<h1>tray（試作）</h1>
<dl>
  <dt>1. 開き方</dt><dd id="mode">調べている。</dd>
  <dt>2. API の要求</dt><dd id="api">調べている。</dd>
</dl>
<h2>3. 枠の中の報告</h2>
<iframe src="${v}/sample.html" sandbox="allow-scripts allow-popups allow-popups-to-escape-sandbox" title="見本の報告"></iframe>
<h2>4. PDF</h2>
<a href="${v}/sample.pdf" target="_blank" rel="noopener">PDF を新しいタブで開く（報告の面のヘッダつき）</a><br>
<a href="${v}/sample.pdf?plain=1" target="_blank" rel="noopener">PDF を新しいタブで開く（sandbox のヘッダなし）</a>
<script>
  var standalone = window.matchMedia('(display-mode: standalone)').matches || navigator.standalone === true;
  document.getElementById('mode').textContent = standalone ? 'ホーム画面から開いた（standalone）。' : 'ブラウザのタブで開いた。';
  fetch('/api/check', { cache: 'no-store' })
    .then(function (r) { return r.ok ? r.json() : Promise.reject(new Error('状態 ' + r.status)); })
    .then(function (j) { document.getElementById('api').textContent = 'Access を通って返った（' + j.at + '）。'; })
    .catch(function (e) { document.getElementById('api').textContent = '落ちた: ' + e.message; });
</script>
</body>
</html>
`;
}

function escape(s) {
  return String(s).replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c]);
}
