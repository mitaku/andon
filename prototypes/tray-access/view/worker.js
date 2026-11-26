// tray の試作の報告の面。Access の外に置き、見本の報告と PDF を ADR-0064 決めたこと 18 のヘッダで返す。
// 試作なので署名付き URL は持たない。
export default {
  async fetch(request, env) {
    if (!env.LIST_ORIGIN) {
      return new Response('LIST_ORIGIN が設定にない。\n', { status: 500, headers: { 'content-type': 'text/plain; charset=utf-8' } });
    }
    const url = new URL(request.url);
    if (url.pathname !== '/sample.html' && url.pathname !== '/sample.pdf') {
      return new Response('無い。\n', { status: 404, headers: { 'content-type': 'text/plain; charset=utf-8' } });
    }
    const asset = await env.ASSETS.fetch(new Request(new URL(url.pathname, url.origin)));
    const headers = new Headers(asset.headers);
    const plain = url.pathname === '/sample.pdf' && url.searchParams.get('plain') === '1';
    if (!plain) {
      headers.set('content-security-policy', [
        "default-src 'none'",
        "script-src 'unsafe-inline' data:",
        "style-src 'unsafe-inline' data:",
        'img-src data: blob:',
        'media-src data: blob:',
        'font-src data:',
        "connect-src 'none'",
        "form-action 'none'",
        "base-uri 'none'",
        `frame-ancestors ${env.LIST_ORIGIN}`,
        'sandbox allow-scripts allow-popups allow-popups-to-escape-sandbox',
      ].join('; '));
    }
    headers.set('x-content-type-options', 'nosniff');
    headers.set('referrer-policy', 'no-referrer');
    headers.set('cache-control', 'no-store');
    headers.set('x-robots-tag', 'noindex');
    return new Response(asset.body, { status: asset.status, headers });
  },
};
