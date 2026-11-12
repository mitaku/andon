# ADR-0021 の改め: pstack の plugin の登録と自動更新は、セルの利用者の settings.json に書かれた状態を正とする

- 日付: 2026-11-12
- 出典: ADR-0021 決めたこと 7（[adr/0021-pstack-in-cell.md](../adr/0021-pstack-in-cell.md)。Chief Engineer の課題 A-0160。人の言葉 2026-11-12「結果としてsettings.jsonに記述されることになると思うのでその状態で良いように思った」）

pstack の plugin の登録と自動更新は、セルの利用者の `~/.claude/settings.json` に2つのキー（`extraKnownMarketplaces` の `cursor-plugins` に `autoUpdate: true`、`enabledPlugins` に `pstack@cursor-plugins`）が書かれた状態を正とする。コマンドで入れても手で書いてもよく、対話の `/plugin` の画面は要らない。「自動更新を非対話で入れられるか」の開いている問いは閉じた。ここで言う `settings.json` をセルの利用者の設定と読んだのは Chief Engineer で、パッケージの `package/settings.json` を指すなら別の決断になる（人の確かめ待ち）。
