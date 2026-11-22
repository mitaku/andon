# ADR-0045: 行燈を Stream Deck のキーに出す

- 日付: 2026-11-22
- 出典: ADR-0045（[adr/0045-stream-deck-lamp.md](../adr/0045-stream-deck-lamp.md)。Chief Engineer の課題。元はオーナーの発言 2026-11-21「StreamDeckにもランプの状態を出したりできるとよさそう」）

Herdr App の左の列の行燈と同じ灯りを、Elgato Stream Deck のキー 1 つに出す。キーには、行燈と同じ色と、待っているものの合計と、内訳を 2 行まで出す。押すと、mod `inbox` が待っているものを順に開き、Herdr App が前に出る。キーから App は起動しない。灯りは mod `inbox` が決めたものを読むだけで、Stream Deck の側では決めない。plugin は、App が書く `shared.json` の `inbox.lamp` を 2 秒ごとに読む。mod `inbox` は `inbox.lamp` に `version` と、値を決めた時刻 `at` を足し、描画を待たずに 30 秒ごとに置き直す。値が読めないか 90 秒より古いとき、キーは「分からない」を出す。App を閉じている間は「分からない」になる。plugin は npm の依存を持たず、Stream Deck の WebSocket の約束を直接話す。`package/host/streamdeck/andon.lamp.sdPlugin/` に置き、`bin/bootstrap host-install` が、置き場を指定されたときだけ手元へ写す。Marketplace には出さない。plugin が herdr-core の API につないで灯りを自分で決める形は取らない。灯りを決める場所を 2 か所にしないためである。herdr-core の API からは、動いている役の端末の数を読んで、2 つ目のキーに出す。タッチ帯とダイヤルは使わない。
