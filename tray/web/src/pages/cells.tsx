// セルと鍵の画面（ADR-0064 決めたこと 19・26）。セルの名前を入れると鍵を1度だけ見せる。サーバは鍵の SHA-256 だけを持つ。
// 同じ画面で鍵を失効させ、作り直せる。直近7日に上がった枚数と開いた枚数も出す（価値の仮説 2）。
import * as stylex from "@stylexjs/stylex";
import {
  AlertDialog,
  AlertDialogBackdrop,
  AlertDialogClose,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogTitle,
} from "@ui-sdk/solid/alert-dialog";
import { Button } from "@ui-sdk/solid/button";
import { Dialog, DialogBackdrop, DialogClose, DialogContent, DialogDescription, DialogTitle } from "@ui-sdk/solid/dialog";
import { Field, FieldLabel } from "@ui-sdk/solid/field";
import { Input } from "@ui-sdk/solid/input";
import { createMemo, createSignal, Errored, For, Loading, refresh, Show } from "solid-js";
import { APIError, type Cell, issueKey, listCells, reissueKey, revokeKey } from "../api";
import { colors } from "../theme.stylex";
import { base, Shell, when } from "../ui";

type Ask = { kind: "reissue" | "revoke"; cell: string };

const KEY_LABEL: Record<Cell["key"], string> = { active: "鍵は有効", revoked: "鍵は失効", none: "鍵は無い" };

const s = stylex.create({
  section: { paddingInline: 16, paddingTop: 16 },
  form: { display: "flex", flexWrap: "wrap", alignItems: "flex-end", gap: 8 },
  field: { display: "flex", flexDirection: "column", gap: 4, flexGrow: 1, minWidth: 180 },
  label: { fontSize: 13, color: colors.mutedForeground },
  input: {
    font: "inherit",
    fontSize: 16,
    paddingBlock: 6,
    paddingInline: 10,
    borderWidth: 1,
    borderStyle: "solid",
    borderColor: colors.border,
    borderRadius: 8,
    backgroundColor: colors.card,
    color: colors.foreground,
    minHeight: 36,
    width: "100%",
  },
  error: { color: colors.destructive, fontSize: 14, marginBlock: 8 },
  list: { listStyle: "none", margin: 0, padding: 0, marginTop: 16 },
  row: {
    display: "flex",
    flexWrap: "wrap",
    alignItems: "center",
    justifyContent: "space-between",
    gap: 8,
    paddingBlock: 12,
    paddingInline: 16,
    borderBottomWidth: 1,
    borderBottomStyle: "solid",
    borderBottomColor: colors.border,
  },
  name: { fontWeight: 600, overflowWrap: "anywhere" },
  meta: { display: "flex", flexWrap: "wrap", columnGap: 8, fontSize: 13, color: colors.mutedForeground },
  actions: { display: "flex", gap: 8 },
  backdrop: { position: "fixed", inset: 0, backgroundColor: "rgb(0 0 0 / 0.4)" },
  dialog: {
    position: "fixed",
    insetInline: 16,
    top: "20%",
    marginInline: "auto",
    maxWidth: 480,
    padding: 20,
    borderRadius: 12,
    borderWidth: 1,
    borderStyle: "solid",
    borderColor: colors.border,
    backgroundColor: colors.card,
    color: colors.foreground,
    display: "flex",
    flexDirection: "column",
    gap: 12,
  },
  dialogTitle: { margin: 0, fontSize: 18 },
  dialogText: { margin: 0, fontSize: 14, color: colors.mutedForeground },
  key: {
    fontFamily: "ui-monospace, SFMono-Regular, Menlo, monospace",
    fontSize: 13,
    overflowWrap: "anywhere",
    userSelect: "all",
    padding: 10,
    borderRadius: 8,
    backgroundColor: colors.muted,
  },
  row2: { display: "flex", justifyContent: "flex-end", gap: 8, flexWrap: "wrap" },
});

export function Cells() {
  const data = createMemo(() => listCells());
  const [name, setName] = createSignal("");
  const [formError, setFormError] = createSignal("");
  const [issued, setIssued] = createSignal<{ cell: string; key: string } | null>(null);
  const [ask, setAsk] = createSignal<Ask | null>(null);
  const [copied, setCopied] = createSignal(false);

  const EXISTS = "このセルはもう鍵を持つ。作り直すなら、一覧の「作り直す」を使う。";

  // issue: 鍵を作り、1度だけ見せる。作る口は有効な鍵があると 409 で断る（一覧が古いときや二重押しでも、前の鍵を残す）。
  // 前の鍵を効かなくするのは、確かめのダイアログを通した作り直し（replace）だけ
  async function issue(cell: string, replace = false) {
    setFormError("");
    try {
      const r = await (replace ? reissueKey(cell) : issueKey(cell));
      setCopied(false);
      setIssued(r);
      return true;
    } catch (e) {
      setFormError(e instanceof APIError && e.status === 409 ? EXISTS : `鍵を作れない: ${(e as Error).message}`);
      return false;
    } finally {
      refresh(data);
    }
  }

  async function create() {
    setFormError("");
    const cell = name().trim();
    if (!cell) return setFormError("セルの名前を入れる。");
    if (data().cells.some((c) => c.cell === cell && c.key === "active")) return setFormError(EXISTS);
    if (await issue(cell)) setName("");
  }

  async function answer(a: Ask) {
    if (a.kind === "revoke") {
      await revokeKey(a.cell);
      refresh(data);
    } else {
      await issue(a.cell, true);
    }
  }

  return (
    <Shell here="/cells">
      <section {...stylex.attrs(s.section)} aria-label="セルを足す">
        <form
          {...stylex.attrs(s.form)}
          onSubmit={(e) => {
            e.preventDefault();
            return create();
          }}
        >
          <Field {...stylex.attrs(s.field)}>
            <FieldLabel {...stylex.attrs(s.label)}>セルの名前</FieldLabel>
            <Input value={name()} onValueChange={(v: string) => setName(v)} autocomplete="off" {...stylex.attrs(s.input, base.focus)} />
          </Field>
          <Button type="submit" {...stylex.attrs(base.button, base.primary, base.focus)}>
            鍵を作る
          </Button>
        </form>
        <Show when={formError()}>
          <p role="alert" {...stylex.attrs(s.error)}>
            {formError()}
          </p>
        </Show>
      </section>

      <Errored fallback={(err) => <p {...stylex.attrs(base.message)}>読めない: {String((err() as Error)?.message ?? err())}</p>}>
        <Loading fallback={<p {...stylex.attrs(base.message)}>読んでいる…</p>}>
          <Show when={data().cells.length > 0} fallback={<p {...stylex.attrs(base.message)}>セルはまだ無い。</p>}>
            <ul {...stylex.attrs(s.list)}>
              <For each={data().cells}>
                {(c) => (
                  <li {...stylex.attrs(s.row)} data-cell={c.cell}>
                    <div>
                      <div {...stylex.attrs(s.name)}>{c.cell}</div>
                      <div {...stylex.attrs(s.meta)}>
                        <span>{KEY_LABEL[c.key]}</span>
                        <Show when={c.created_at}>{(at) => <span>作った日時 {when(at())}</span>}</Show>
                        <span>
                          直近{data().days}日: 上がった {c.uploads} 枚・開いた {c.opens} 枚
                        </span>
                      </div>
                    </div>
                    <div {...stylex.attrs(s.actions)}>
                      <Show
                        when={c.key === "none"}
                        fallback={
                          <button
                            type="button"
                            onClick={() => setAsk({ kind: "reissue", cell: c.cell })}
                            {...stylex.attrs(base.button, base.focus)}
                          >
                            作り直す
                          </button>
                        }
                      >
                        <Button onClick={() => issue(c.cell)} {...stylex.attrs(base.button, base.focus)}>
                          鍵を作る
                        </Button>
                      </Show>
                      <Show when={c.key === "active"}>
                        <button
                          type="button"
                          onClick={() => setAsk({ kind: "revoke", cell: c.cell })}
                          {...stylex.attrs(base.button, base.focus)}
                        >
                          失効させる
                        </button>
                      </Show>
                    </div>
                  </li>
                )}
              </For>
            </ul>
          </Show>
        </Loading>
      </Errored>

      <AlertDialog open={ask() !== null} onOpenChange={(open: boolean) => !open && setAsk(null)}>
        <AlertDialogBackdrop {...stylex.attrs(s.backdrop)} />
        <AlertDialogContent {...stylex.attrs(s.dialog)}>
          <AlertDialogTitle {...stylex.attrs(s.dialogTitle)}>
            {ask()?.kind === "revoke" ? `${ask()?.cell} の鍵を失効させる？` : `${ask()?.cell} の鍵を作り直す？`}
          </AlertDialogTitle>
          <AlertDialogDescription {...stylex.attrs(s.dialogText)}>
            {ask()?.kind === "revoke"
              ? "このセルは報告を上げられなくなる。作り直すまで戻せない。"
              : "前の鍵は効かなくなる。新しい鍵をセルに置き直すまで、このセルは報告を上げられない。"}
          </AlertDialogDescription>
          <div {...stylex.attrs(s.row2)}>
            <AlertDialogClose {...stylex.attrs(base.button, base.focus)}>やめる</AlertDialogClose>
            <AlertDialogClose
              value="confirm"
              action={() => {
                const a = ask();
                return a ? answer(a) : undefined;
              }}
              {...stylex.attrs(base.button, base.danger, base.focus)}
            >
              {ask()?.kind === "revoke" ? "失効させる" : "作り直す"}
            </AlertDialogClose>
          </div>
        </AlertDialogContent>
      </AlertDialog>

      <Dialog open={issued() !== null} onOpenChange={(open: boolean) => !open && setIssued(null)} dismissible={false}>
        <DialogBackdrop {...stylex.attrs(s.backdrop)} />
        <DialogContent {...stylex.attrs(s.dialog)}>
          <DialogTitle {...stylex.attrs(s.dialogTitle)}>{issued()?.cell} の鍵</DialogTitle>
          <DialogDescription {...stylex.attrs(s.dialogText)}>
            鍵はこの1度だけ見せる。閉じると、もう見られない。セルで andon tray auth に stdin で渡す。
          </DialogDescription>
          <code {...stylex.attrs(s.key)} data-testid="issued-key">
            {issued()?.key}
          </code>
          <div {...stylex.attrs(s.row2)}>
            <button
              type="button"
              onClick={async () => {
                const k = issued()?.key;
                if (!k) return;
                try {
                  await navigator.clipboard.writeText(k);
                  setCopied(true);
                } catch {
                  setCopied(false);
                }
              }}
              {...stylex.attrs(base.button, base.focus)}
            >
              {copied() ? "写した" : "写す"}
            </button>
            <DialogClose {...stylex.attrs(base.button, base.primary, base.focus)}>閉じる</DialogClose>
          </div>
        </DialogContent>
      </Dialog>
    </Shell>
  );
}
