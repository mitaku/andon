// 報告の一覧（ADR-0064 決めたこと 21）。セルとプロジェクトでまとめ、まとめも行も新しい順に並べる。
// 開いたときの絞り込みは未読（/）。30日（/all）と古い報告（/old）は帯で切り替える。まとめて既読にする口は無い。
import * as stylex from "@stylexjs/stylex";
import { Select, SelectContent, SelectItem, SelectItemText, SelectTrigger, SelectValue } from "@ui-sdk/solid/select";
import { createMemo, createSignal, Errored, For, Loading, Show } from "solid-js";
import { listReports, type Report, type View } from "../api";
import { colors } from "../theme.stylex";
import { base, Shell, when } from "../ui";

const HERE: Record<View, string> = { unread: "/", recent: "/all", old: "/old" };
const EMPTY: Record<View, string> = {
  unread: "未読の報告は無い。",
  recent: "30日のうちに上がった報告は無い。",
  old: "古い報告は無い。最後に上がってから30日を過ぎた報告がここに移る。",
};
const ALL = "*";

export interface Group {
  key: string;
  cell: string;
  project: string;
  reports: Report[];
}

// groupReports: セルとプロジェクトの組でまとめる。まとめは中の最新の報告が新しい順、行も新しい順
export function groupReports(reports: Report[]): Group[] {
  const by = new Map<string, Group>();
  for (const r of reports) {
    const key = `${r.cell}/${r.project}`;
    let g = by.get(key);
    if (!g) {
      g = { key, cell: r.cell, project: r.project, reports: [] };
      by.set(key, g);
    }
    g.reports.push(r);
  }
  const groups = [...by.values()];
  for (const g of groups) g.reports.sort((a, b) => b.updated_at.localeCompare(a.updated_at));
  return groups.sort((a, b) => (b.reports[0]?.updated_at ?? "").localeCompare(a.reports[0]?.updated_at ?? ""));
}

const s = stylex.create({
  bar: { display: "flex", alignItems: "center", gap: 8, paddingInline: 16, paddingTop: 12 },
  trigger: {
    display: "inline-flex",
    alignItems: "center",
    justifyContent: "space-between",
    gap: 8,
    minWidth: 200,
    maxWidth: "100%",
  },
  content: {
    backgroundColor: colors.card,
    color: colors.foreground,
    borderWidth: 1,
    borderStyle: "solid",
    borderColor: colors.border,
    borderRadius: 8,
    padding: 4,
    margin: 0,
    minWidth: "var(--anchor-width)",
    maxHeight: "var(--available-height)",
    overflowY: "auto",
    boxShadow: "0 8px 24px rgb(0 0 0 / 0.12)",
  },
  item: {
    paddingBlock: 6,
    paddingInline: 10,
    borderRadius: 6,
    cursor: "default",
    fontSize: 14,
    backgroundColor: { default: "transparent", ":is([data-highlighted])": colors.muted },
    fontWeight: { default: 400, ":is([data-selected])": 600 },
  },
  group: { marginTop: 16 },
  heading: {
    fontSize: 13,
    fontWeight: 600,
    color: colors.mutedForeground,
    margin: 0,
    paddingInline: 16,
    paddingBlock: 4,
  },
  list: { listStyle: "none", margin: 0, padding: 0 },
  row: {
    display: "block",
    paddingBlock: 10,
    paddingInline: 16,
    color: "inherit",
    textDecoration: "none",
    borderBottomWidth: 1,
    borderBottomStyle: "solid",
    borderBottomColor: colors.border,
    backgroundColor: { default: "transparent", ":hover": colors.muted },
  },
  title: { display: "flex", alignItems: "baseline", gap: 8, overflowWrap: "anywhere" },
  unreadTitle: { fontWeight: 700 },
  dot: {
    flexShrink: 0,
    width: 8,
    height: 8,
    borderRadius: 999,
    backgroundColor: colors.primary,
    transform: "translateY(-1px)",
  },
  meta: { display: "flex", flexWrap: "wrap", columnGap: 8, fontSize: 13, color: colors.mutedForeground },
});

export function ReportList(props: { view: View }) {
  const reports = createMemo(() => listReports(props.view));
  const [filter, setFilter] = createSignal<string>(ALL);
  const groups = createMemo(() => groupReports(reports()));
  const shown = createMemo(() => (filter() === ALL ? groups() : groups().filter((g) => g.key === filter())));
  const items = createMemo(() => [
    { value: ALL, label: "全部のセルとプロジェクト" },
    ...groups().map((g) => ({ value: g.key, label: `${g.cell} / ${g.project}` })),
  ]);
  const from = props.view === "unread" ? "" : `?from=${props.view === "old" ? "old" : "all"}`;

  return (
    <Shell here={HERE[props.view]}>
      <Errored fallback={(err) => <p {...stylex.attrs(base.message)}>読めない: {String((err() as Error)?.message ?? err())}</p>}>
        <Loading fallback={<p {...stylex.attrs(base.message)}>読んでいる…</p>}>
          <Show when={groups().length > 0} fallback={<p {...stylex.attrs(base.message)}>{EMPTY[props.view]}</p>}>
            <div {...stylex.attrs(s.bar)}>
              <Select
                items={items()}
                value={filter()}
                onValueChange={(v: unknown) => setFilter(typeof v === "string" ? v : ALL)}
                aria-label="セルとプロジェクトで絞る"
              >
                <SelectTrigger {...stylex.attrs(base.button, base.focus, s.trigger)} data-testid="group-filter">
                  <SelectValue />
                  <span aria-hidden="true">▾</span>
                </SelectTrigger>
                <SelectContent {...stylex.attrs(s.content)}>
                  <For each={items()}>
                    {(it) => (
                      <SelectItem value={it.value} {...stylex.attrs(s.item)}>
                        <SelectItemText>{it.label}</SelectItemText>
                      </SelectItem>
                    )}
                  </For>
                </SelectContent>
              </Select>
            </div>
            <For each={shown()}>
              {(g) => (
                <section {...stylex.attrs(s.group)} aria-label={`${g.cell} / ${g.project}`}>
                  <h2 {...stylex.attrs(s.heading)}>
                    {g.cell} / {g.project}
                  </h2>
                  <ul {...stylex.attrs(s.list)}>
                    <For each={g.reports}>
                      {(r) => (
                        <li>
                          <a href={`/r/${r.id}${from}`} {...stylex.attrs(s.row, base.focus)} data-unread={r.unread ? "" : undefined}>
                            <span {...stylex.attrs(s.title, r.unread && s.unreadTitle)}>
                              <Show when={r.unread}>
                                <span {...stylex.attrs(s.dot)} role="img" aria-label="未読" />
                              </Show>
                              <span>{r.title || r.name}</span>
                            </span>
                            <span {...stylex.attrs(s.meta)}>
                              <time datetime={r.updated_at}>{when(r.updated_at)}</time>
                              <span>{r.cell}</span>
                              <span>{r.project}</span>
                            </span>
                          </a>
                        </li>
                      )}
                    </For>
                  </ul>
                </section>
              )}
            </For>
          </Show>
        </Loading>
      </Errored>
    </Shell>
  );
}
