# 版ごとの変更は CHANGELOG.md に、版を切るときだけ書く

- 日付: 2026-11-07
- 出典: 人（2026-11-07「v0.0.4 のタグを打つときにリリースノートというか、CHANGELOG を作っておきたい」）

版ごとの「変わったこと」と「上げるときにすること」を、repo の根の [CHANGELOG.md](../../CHANGELOG.md) に書く。

- 書くのは版を切るときだけ。手順は「CHANGELOG の節を足す PR → マージ → annotated tag」（[operate.md](../operate.md) §14）。
- PR ごとには触らない。

捨てた案:

- **全部の PR が CHANGELOG を触る**: 並べて走る PR がどれも同じ所を触り、ぶつかる（[design.md](../design.md) §5 が yard-setup の形として捨てたもの）。
