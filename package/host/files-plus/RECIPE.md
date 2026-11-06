# Files plus

Herdr's Files panel with picture and diagram previews: browse the focused pane's folder, click an image or a Mermaid file and see it, also when the folder is on another machine.

## What you see

Herdr's Files panel as shipped: a 42px header (back arrow, "Files", the file's path), the folder tree, git marks. A picked image shows centered under the header, at most the panel's width, scrolling if tall. While it loads: "Loading file…"; on failure the reason in `dim` (for example "Too large to preview (6.2 MiB)").

## How it behaves

- Image extensions: png, jpg, jpeg, gif, webp, bmp, svg (case-insensitive). Other files behave as in Herdr's Files.
- The picture is copied into `cache/files-plus/<ms>-<hash>.<ext>` under the app home and drawn with `image()`. The hash is FNV-1a over machine, path, size and modification time; a cached file with the same hash is reused.
- Mermaid files (`.mmd`, `.mermaid`): read with `readTextFile(path, {on})`, drawn with Pele's `render()` in the mod, CSS variables in the SVG replaced by theme colors (`--pele-bg` background, `-fg` and `-line` foreground, `-muted` muted, `-surface` surface, `-surface-alt` raised, `-border` border, `-accent` accent), root `width`/`height` rewritten to fit the panel, and written as text to the cache (`<ms>-<hash>.svg`, hash of text, theme and width). Errors show up to 4 lines of Pele's message.
- Markdown (`.md`, `.markdown`): opens rendered; a 11.5px **Source** / **Preview** text button after "Files" switches to Herdr's source view and back. The text is cut into blocks in order: Markdown runs for `TextView.markdown` (selectable, links handled), ```` ```mermaid ```` fences drawn like Mermaid files, and lines that are only `![alt](path)` (a relative path) shown as pictures found relative to the Markdown file. Fences of other languages, and image-looking lines inside them, stay Markdown. Blocks are 10px apart, the column has 14px padding; a block still loading reads "Drawing diagram…" / "Loading image…", a failed one shows its message on `surface`.
- Links in Markdown: `http(s)` URLs open with `explorer.exe` / `open` / `xdg-open` (nothing else is passed); other links resolve against the Markdown file (or the folder root, for absolute paths inside it) and open in the panel; a link leaving the folder does nothing.
- At start, cache files older than 7 days and all but the newest 50 are removed.

## Built from

- A copy of Herdr's `files` mod (Apache-2.0, LICENSE beside it), with asset paths moved to `mods/files-plus/`.
- Size and time: `Get-Item` through PowerShell here on Windows, `stat -c "%s %Y"` elsewhere (with `{on}` for another machine).
- Bytes: `Copy-Item` here on Windows; from another machine `base64 -w0`, written as text with `fs.writeFile`, decoded here (`[Convert]::FromBase64String` + `WriteAllBytes` on Windows, `base64 -d` elsewhere).

## Watch out

- `TextView.markdown` draws no images, neither app-home-relative nor absolute: cut images out as their own blocks.
- `on_link_click` replaces TextView's own link opening, so web links need opening by hand; pass only `http(s)` URLs to the system opener.
- Herdr's SVG drawing ignores CSS variables: Pele's raw SVG draws black shapes. Resolve them first.
- `image()` draws an SVG at the size its root element says; `.w()`/`.h()` don't scale it. Write the size into the file.
- `on_size_change` reports the panel as shown; sizes a mod sets are multiplied by the app's zoom (`settings().zoom`). Divide by it.
- The panel's width can arrive after a diagram was drawn: draw again when the width used differs.

- In herdr-app 0.1.10 `fs.writeFile` with a `Uint8Array` (or `Buffer`) writes an empty file; strings work. Hence the programs above.
- `image()` takes a path relative to the app home, not an absolute path.
- `exec` output is capped at 8 MiB, so base64 allows about 6 MiB; the limit is 5 MiB.
- Keep the block id `files` in layouts: the tab bar's Files button opens the panel by that id.

## Done when

- `herdr-app check` passes.
- Clicking a png in a local folder shows it; clicking one in a folder on another machine shows it.
- A picture over 5 MiB shows "Too large to preview".
