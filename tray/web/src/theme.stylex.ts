// 一覧の面の色（ADR-0064 決めたこと 24）。オーナーのデザインシステムの色の値（OKLCH）を写して StyleX の変数にした。
// light と dark の2組を prefers-color-scheme で切り替える。light-dark() は使わない（ビルドの道具が古い書き方に直すことがある）。
// 値を変えるときは、デザインシステムの側の値から写し直す。ここで値を作らない。
import * as stylex from "@stylexjs/stylex";

const DARK = "@media (prefers-color-scheme: dark)";

export const colors = stylex.defineVars({
  background: { default: "oklch(0.971 0.011 84)", [DARK]: "oklch(0.2 0.028 258)" },
  foreground: { default: "oklch(0.235 0.018 58)", [DARK]: "oklch(0.928 0.012 86)" },
  card: { default: "oklch(0.986 0.008 85)", [DARK]: "oklch(0.245 0.03 258)" },
  muted: { default: "oklch(0.938 0.011 80)", [DARK]: "oklch(0.3 0.028 258)" },
  mutedForeground: { default: "oklch(0.515 0.025 66)", [DARK]: "oklch(0.715 0.022 82)" },
  primary: { default: "oklch(0.42 0.108 252)", [DARK]: "oklch(0.545 0.135 256)" },
  primaryForeground: { default: "oklch(0.98 0.01 85)", [DARK]: "oklch(0.97 0.012 86)" },
  secondary: { default: "oklch(0.928 0.013 80)", [DARK]: "oklch(0.3 0.03 258)" },
  secondaryForeground: { default: "oklch(0.305 0.05 254)", [DARK]: "oklch(0.928 0.012 86)" },
  accent: { default: "oklch(0.918 0.028 250)", [DARK]: "oklch(0.33 0.042 256)" },
  accentForeground: { default: "oklch(0.31 0.07 254)", [DARK]: "oklch(0.95 0.012 86)" },
  destructive: { default: "oklch(0.553 0.171 32)", [DARK]: "oklch(0.57 0.165 33)" },
  destructiveForeground: { default: "oklch(0.985 0.01 85)", [DARK]: "oklch(0.98 0.01 86)" },
  border: { default: "oklch(0.882 0.012 80)", [DARK]: "oklch(0.83 0.04 255 / 12%)" },
  ring: { default: "oklch(0.55 0.1 252)", [DARK]: "oklch(0.6 0.12 256)" },
});
