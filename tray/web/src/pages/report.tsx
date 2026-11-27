// 報告の画面（ADR-0064 決めたこと 18・20・22）。上に戻る口だけを持つ細い帯を置き、その下の枠に別オリジンの報告の面を出す。
// 開くと一覧の面の API（POST /api/reports/<id>/open）が既読を書き、5分で切れる署名付き URL を返す。報告に見た目を被せない。
// PDF は枠に入れず新しいタブで開き、開かない形式はダウンロードの口を出す（どちらも開く口を押したときに URL を使う）。
import { useLocation, useParams } from "@solidjs/router";
import * as stylex from "@stylexjs/stylex";
import { createMemo, Errored, Loading, Match, refresh, Switch } from "solid-js";
import { openReport } from "../api";
import { colors } from "../theme.stylex";
import { base } from "../ui";

// 枠の要素にも sandbox を付ける。allow-same-origin と allow-top-navigation は付けない（報告のスクリプトが枠を外のページへ
// 移しても、移った先は sandbox の中に居る。報告の面の CSP の sandbox と同じ組）
export const FRAME_SANDBOX = "allow-scripts allow-popups allow-popups-to-escape-sandbox";

const BACK: Record<string, [string, string]> = {
  all: ["/all", "30日"],
  old: ["/old", "古い報告"],
};

const s = stylex.create({
  page: { display: "flex", flexDirection: "column", height: "100dvh" },
  bar: {
    display: "flex",
    alignItems: "center",
    gap: 12,
    flexShrink: 0,
    paddingTop: "max(6px, env(safe-area-inset-top))",
    paddingBottom: 6,
    paddingInline: 12,
    borderBottomWidth: 1,
    borderBottomStyle: "solid",
    borderBottomColor: colors.border,
    backgroundColor: colors.background,
    fontSize: 14,
  },
  back: { color: colors.primary, textDecoration: "none", whiteSpace: "nowrap", paddingBlock: 4 },
  title: { overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", color: colors.mutedForeground },
  frame: { display: "block", flexGrow: 1, width: "100%", borderWidth: 0 },
  panel: { paddingBlock: 24, paddingInline: 16, display: "flex", flexDirection: "column", gap: 12, alignItems: "flex-start" },
});

export function ReportFrame() {
  const params = useParams();
  const location = useLocation();
  const opened = createMemo(() => openReport(params.id ?? ""));
  const back = createMemo(() => BACK[new URLSearchParams(location.search).get("from") ?? ""] ?? ["/", "一覧"]);

  return (
    <div {...stylex.attrs(s.page)}>
      <div {...stylex.attrs(s.bar)}>
        <a href={back()[0]} {...stylex.attrs(s.back, base.focus)}>
          ← {back()[1]}
        </a>
        <Loading fallback={null}>
          <Errored fallback={null}>
            <span {...stylex.attrs(s.title)}>{opened().title}</span>
          </Errored>
        </Loading>
      </div>
      <Errored fallback={(err) => <p {...stylex.attrs(base.message)}>開けない: {String((err() as Error)?.message ?? err())}</p>}>
        <Loading fallback={<p {...stylex.attrs(base.message)}>開いている…</p>}>
          <Switch>
            <Match when={opened().open_as === "frame"}>
              <iframe src={opened().url} title={opened().title} sandbox={FRAME_SANDBOX} {...stylex.attrs(s.frame)} />
            </Match>
            <Match when={opened().open_as === "tab"}>
              <div {...stylex.attrs(s.panel)}>
                <p {...stylex.attrs(base.muted)}>PDF は新しいタブで開く。リンクは5分で切れる。</p>
                <a href={opened().url} target="_blank" rel="noopener noreferrer" {...stylex.attrs(base.button, base.primary, base.focus)}>
                  PDF を新しいタブで開く
                </a>
                <button type="button" onClick={() => refresh(opened)} {...stylex.attrs(base.button, base.focus)}>
                  リンクを作り直す
                </button>
              </div>
            </Match>
            <Match when={opened().open_as === "download"}>
              <div {...stylex.attrs(s.panel)}>
                <p {...stylex.attrs(base.muted)}>
                  この形式（{opened().content_type}）は tray で開かない。ダウンロードする。リンクは5分で切れる。
                </p>
                <a href={opened().url} rel="noopener noreferrer" {...stylex.attrs(base.button, base.primary, base.focus)}>
                  ダウンロードする
                </a>
                <button type="button" onClick={() => refresh(opened)} {...stylex.attrs(base.button, base.focus)}>
                  リンクを作り直す
                </button>
              </div>
            </Match>
          </Switch>
        </Loading>
      </Errored>
    </div>
  );
}
