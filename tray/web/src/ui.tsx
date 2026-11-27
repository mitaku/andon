// 画面に共通の枠と見た目。スマホの1列を先に作る（決めたこと 21）。
import * as stylex from "@stylexjs/stylex";
import type { JSX } from "@solidjs/web";
import { colors } from "./theme.stylex";

export const base = stylex.create({
  root: {
    backgroundColor: colors.background,
    color: colors.foreground,
    fontFamily: 'system-ui, -apple-system, "Hiragino Sans", "Noto Sans JP", sans-serif',
    fontSize: 16,
    lineHeight: 1.6,
    minHeight: "100dvh",
  },
  focus: {
    outline: { default: "none", ":focus-visible": `2px solid ${colors.ring}` },
    outlineOffset: 2,
  },
  button: {
    appearance: "none",
    borderWidth: 1,
    borderStyle: "solid",
    borderColor: colors.border,
    borderRadius: 8,
    backgroundColor: { default: colors.card, ":hover": colors.muted },
    color: colors.foreground,
    font: "inherit",
    fontSize: 14,
    paddingBlock: 6,
    paddingInline: 12,
    cursor: "pointer",
    minHeight: 36,
  },
  primary: {
    borderColor: colors.primary,
    backgroundColor: { default: colors.primary, ":hover": colors.primary },
    color: colors.primaryForeground,
  },
  danger: {
    borderColor: colors.destructive,
    backgroundColor: { default: colors.destructive, ":hover": colors.destructive },
    color: colors.destructiveForeground,
  },
  link: {
    color: colors.primary,
    textDecoration: "underline",
    textUnderlineOffset: 3,
  },
  muted: { color: colors.mutedForeground, fontSize: 14 },
  message: { paddingBlock: 24, paddingInline: 16, color: colors.mutedForeground, margin: 0 },
});

const shell = stylex.create({
  header: {
    position: "sticky",
    top: 0,
    zIndex: 1,
    display: "flex",
    alignItems: "center",
    gap: 4,
    paddingTop: "max(8px, env(safe-area-inset-top))",
    paddingBottom: 8,
    paddingInline: 12,
    backgroundColor: colors.background,
    borderBottomWidth: 1,
    borderBottomStyle: "solid",
    borderBottomColor: colors.border,
    overflowX: "auto",
  },
  brand: { fontWeight: 700, marginInlineEnd: 8, letterSpacing: "0.02em" },
  tab: {
    color: colors.mutedForeground,
    textDecoration: "none",
    whiteSpace: "nowrap",
    fontSize: 14,
    paddingBlock: 6,
    paddingInline: 10,
    borderRadius: 999,
    backgroundColor: { default: "transparent", ":hover": colors.muted },
  },
  current: { backgroundColor: colors.accent, color: colors.accentForeground, fontWeight: 600 },
  main: { maxWidth: 720, marginInline: "auto", paddingBottom: "env(safe-area-inset-bottom)" },
});

const TABS: [string, string][] = [
  ["/", "未読"],
  ["/all", "30日"],
  ["/old", "古い報告"],
  ["/cells", "セル"],
];

// Shell: 一覧・古い報告・セルの画面の枠。上に画面を切り替える帯を置く
export function Shell(props: { here: string; children: JSX.Element }) {
  return (
    <>
      <header {...stylex.attrs(shell.header)}>
        <span {...stylex.attrs(shell.brand)}>tray</span>
        <nav aria-label="画面">
          {TABS.map(([href, label]) => (
            <a
              href={href}
              aria-current={props.here === href ? "page" : undefined}
              {...stylex.attrs(shell.tab, base.focus, props.here === href && shell.current)}
            >
              {label}
            </a>
          ))}
        </nav>
      </header>
      <main {...stylex.attrs(shell.main)}>{props.children}</main>
    </>
  );
}

// when: 日時を短く出す。今年なら年を省く
export function when(iso: string): string {
  const d = new Date(iso);
  const sameYear = d.getFullYear() === new Date().getFullYear();
  return d.toLocaleString("ja-JP", {
    ...(sameYear ? {} : { year: "numeric" }),
    month: "numeric",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}
