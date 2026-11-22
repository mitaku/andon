// What the Lamp key shows for a value of inbox.lamp, as an SVG (ADR-0045 decisions 6 and 7).
// The inbox mod decides the light. This only tells whether the value can still be trusted, and draws it.

// The only version of inbox.lamp this plugin knows (../../inbox/README.md has the fields).
const VERSION = 1;
// Three times the 30 seconds the inbox takes to put its value again.
const FRESH = 90000;
// The sidebar lantern's colors (../../andon/brand.js).
const LIT = {
  amber: { lamp: "#F0A202", ink: "#1F2328" },
  red: { lamp: "#D8453B", ink: "#FFFFFF" },
};
const DARK = { ground: "#1F2328", frame: "#ECE6DA", window: "#454B53" };
// What waits, the most pressing first, and the word the key says for it.
const KINDS = [["failed", "failed"], ["blocked", "blocked"], ["decision", "decide"], ["merge", "merge"], ["reply", "reply"]];
const LINES = 2;
// Stream Deck draws SVG with Qt, which takes one family name, not a list.
const FONT = 'font-family="Arial" font-weight="bold" text-anchor="middle"';

// The value as the key may show it, or null when it can't tell ("?").
export function trusted(value, now) {
  if (value?.version !== VERSION) return null;
  // === on the names, not a lookup: a lookup would also take ["red"], which turns into "red".
  if (value.light !== null && value.light !== "amber" && value.light !== "red") return null;
  if (!Number.isFinite(value.at) || Math.abs(now - value.at) > FRESH) return null;
  return value;
}

const count = (n) => (Number.isSafeInteger(n) && n > 0 ? n : 0);

// The logo's lantern (docs/assets/logo/andon-mark-dark.svg) with its window unlit.
function lantern(x, y, scale) {
  return `<g transform="translate(${x} ${y}) scale(${scale})">`
    + `<path d="M26 4H38L57 60H45L42 50H22L19 60H7Z" fill="${DARK.frame}"/>`
    + `<path d="M29.5 12H34.5L45 42H19Z" fill="${DARK.window}"/></g>`;
}

const text = (x, y, size, fill, said) => `<text x="${x}" y="${y}" font-size="${size}" fill="${fill}" ${FONT}>${said}</text>`;

function lit(value) {
  const { lamp, ink } = LIT[value.light];
  const waiting = KINDS.map(([field, word]) => [count(value[field]), word]).filter(([n]) => n > 0);
  const total = waiting.reduce((sum, [n]) => sum + n, 0);
  const lines = waiting.slice(0, LINES).map(([n, word], i) => text(72, 104 + i * 26, 22, ink, `${n} ${word}`));
  return [lamp, text(72, 72, total < 1000 ? 64 : 44, ink, total) + lines.join("")];
}

// The key's image for what trusted() returned.
export function draw(value) {
  let ground = DARK.ground;
  let body;
  if (!value) body = lantern(4, 32, 1.25) + text(114, 98, 72, DARK.frame, "?");
  else if (value.light === null) body = lantern(24, 24, 1.5);
  else [ground, body] = lit(value);
  const svg = '<svg xmlns="http://www.w3.org/2000/svg" width="144" height="144" viewBox="0 0 144 144">'
    + `<rect width="144" height="144" fill="${ground}"/>${body}</svg>`;
  return `data:image/svg+xml,${encodeURIComponent(svg)}`;
}
