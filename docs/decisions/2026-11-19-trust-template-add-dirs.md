# boot はプロファイルが --add-dir で渡すフォルダも Claude Code で信頼済みにする

- 日付: 2026-11-19
- 出典: オーナーの決定 2026-11-06（セルでは信頼の画面を出さない。decisions.md §16 の 2026-11-06 の行）と、2026-11-19 にセルで測った結果（docs/image.md）

- boot と、起動する直前の `andon issue start`・`andon chief-engineer` は、repo の根に加えて、配布するプロファイル（パッケージの `templates/` とセルの `templates.local/`。同じ名前なら `templates.local` が勝つ）の frontmatter の `command:`・`resume:` の行が `--add-dir` で渡すフォルダを、Claude Code の `.claude.json` で信頼済みにする。本文の `/opt/andon` は boot がプロファイルを生成するときと同じく `ANDON_HOME` に置き換えて読む。
- 足すのは渡したフォルダそのものだけで、親は足さない。実在しないフォルダと相対パスは足さない。オーナーが hc に直接入れたプロファイルは読まない。
- Codex には足さない。Codex のプロファイルは `--add-dir` を使わず、git の外では信頼の画面をもともと出さない。
- 理由: Claude Code は `--add-dir` のフォルダの信頼も確かめる。git の外にあるフォルダ（Chief of Staff の `--add-dir /opt/andon/skills/chief-of-staff/playbooks`）は、起動のたびに信頼の画面を出す。画面で答えても、その答えはそのセッションだけで残らない。repo の中のフォルダ（`/workspace/wiki`）は repo の根の信頼が効くので画面は出ないが、同じ手で足しても範囲は広がらない。
