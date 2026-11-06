# files-plus

Herdr's Files panel, plus pictures and diagrams. Click a `png`, `jpg`, `gif`, `webp`, `bmp` or `svg` file and the panel shows the image instead of "Binary file · no text preview"; click a Mermaid file (`.mmd`, `.mermaid`) and it shows the diagram; click a Markdown file (`.md`, `.markdown`) and it shows it rendered, with its Mermaid blocks drawn and its image lines shown; click an HTML file (`.html`, `.htm`) and it shows the page as text, with **Open in browser** for the page as made. All of it works on this computer and on other machines. Everything else (the tree, git marks, text previews) is Herdr's shipped `files` mod as it was.

## Place it

The `andon` layout (`../layouts/andon.json`) uses it for the Files panel. In your own copy of `main`, change the panel's Files block to it, keeping the id `files` so the tab bar's Files button still opens it:

```json
{ "id": "files", "type": "block", "content": { "mod": "files-plus" } }
```

## How pictures get here

`image()` only draws files under the herdr-app home, so a picture is copied into `cache/files-plus/` first:

- **This computer (Windows):** PowerShell copies the file.
- **Another machine:** `stat` and `base64` run there; the text is written here and PowerShell decodes it.
- Pictures over 5 MiB are not fetched ("Too large to preview").
- The cache is named by machine, path, size and time, so an edited picture is fetched again. Pictures older than a week, and all but the newest 50, are removed when the panel starts. The cache is plain files in the app home: don't preview pictures you wouldn't leave there.

## Diagrams

Mermaid files are drawn inside the mod by [Pele](https://github.com/obsidianmd/pele) (bundled in `pele/`), in Herdr's theme colors and scaled down to the panel's width. A file with a mistake shows Pele's message (the line and what it expected). The drawing goes into `cache/files-plus/` like a picture; a change to the file, the theme or the panel's width draws it again.

## Markdown

Markdown opens rendered (headings, lists, task lists, tables, quotes, code). **Source** in the header switches to Herdr's source view and **Preview** back. Herdr's Markdown view draws no images, so the text is cut into blocks:

- ```` ```mermaid ```` fences are drawn as diagrams.
- A line that is only an image (`![alt](docs/shot.png)`) shows the picture, found relative to the Markdown file. Images inside a sentence, and images from the web, are not shown.
- Links: `http(s)` links open in your browser; links to files in this folder (`other.md`, `../docs/x.md#part`) open here.

## HTML

Meant for documents written for people (a page an agent made to put a decision in front of you), while agents read Markdown.

- The page is shown as text by Herdr's HTML view: headings, paragraphs, lists, tables, links, code. **CSS is not supported there**, so the page's head, `<style>`, `<script>`, `<template>`, `<noscript>` and comments are dropped first, and a line above says so. Pages over 400,000 characters show only the source. **Source** / **Preview** switch as for Markdown.
- **Open in browser** shows the page as made, with its styles and scripts: on this computer, the file itself (with the files beside it); from another machine, a copy in `cache/files-plus/` (files beside it, such as its pictures or stylesheets, don't come along; over 5 MiB is not copied). **The page's scripts run in your browser**, which is why it only opens on a click.
- Links in the page work as in Markdown.

## What it runs

Herdr's own calls are unchanged (`exec` of PowerShell or `sh` to list folders, `git rev-parse` / `git status`, `readTextFile`, `watchFolder`). files-plus adds, only when an image is clicked:

| Call | Where |
|---|---|
| `exec("powershell", ["-NoProfile", "-NonInteractive", "-Command", <fixed script> followed by each local path as a single-quoted string (`'`, `‘`, `’`, `‚`, `‛` doubled)])`: `Get-Item` (size, time), `Copy-Item`, `[IO.File]::WriteAllBytes` of decoded base64 | this computer (Windows) |
| `exec("stat", ["-c", "%s %Y", "--", path], {on})`, `exec("base64", ["-w0", "--", path], {on})` | the machine the file is on (read only) |
| `exec("sh", ["-c", 'base64 -d "$1" > "$2"', …])` | this computer, when it isn't Windows |
| `readTextFile(path, {on})` of a `.mmd` / `.mermaid` file, then Pele in the mod (no program runs) | where the file is |
| `exec("explorer.exe" \| "open" \| "xdg-open", [url])` when an `http(s)` link in Markdown is clicked; only `http://` / `https://` URLs | this computer |
| `exec("explorer.exe" \| "open" \| "xdg-open", [path])` when **Open in browser** is clicked on an `.html` / `.htm` file: the file's path here, or its copy in the cache (copied as a picture is) | this computer |
| `fs.mkdir` / `readdir` / `writeFile` (base64 text, diagram SVG) / `unlink` | `cache/files-plus/` in the app home |

Before any of these reads (a picture, a diagram file, a page copied to open, a picture a Markdown file names), files-plus checks on the machine the file is on that neither the file nor any folder between the panel's folder and it is a symbolic link (a reparse point on Windows): `exec("sh", ["-c", <fixed script>, "files-plus", <folder>, <each part of the path>…], {on})`, or on this computer with Windows the same in PowerShell with quoted arguments. A link is not read ("is or goes through a symbolic link: not read"); a repository can hold a link to a file outside it. Diagram files over 1 MiB are not drawn. The text preview of a file you click is Herdr's and follows links as before.

No network, no credentials, nothing written on other machines.

## Origin

Copied from herdr-flightdeck (a private repository that Andon came out of) `herdr-app/mods/files-plus` at commit `afc57ac5ec85d3123cbf5c067a9cd4ee3cf5b268`. Changed for Andon: the PowerShell quoting above, refusing symbolic links before its own reads, the layout's name. The preview picture was not copied.

## Licence

`main.js` is a changed copy of Herdr's shipped `files` mod (herdr-app 0.1.10, `defaults/mods/files` in [herdrdev/herdr-app](https://github.com/herdrdev/herdr-app)) by the herdr-app authors, under the Apache License 2.0: see [LICENSE](LICENSE). The icons are Herdr's, unchanged. The changes are marked at the top of `main.js`: the previews (made in herdr-flightdeck (a private repository that Andon came out of)), and Andon's quoting of PowerShell arguments, which also covers Herdr's own folder listing.

`pele/pele.js` is [Pele](https://github.com/obsidianmd/pele) 1.13.0 by kepano, MIT (`pele/LICENSE`), exactly as published on npm: `dist/pele.js` from `pele-1.13.0.tgz` (npm integrity `sha512-Ie8UKY9x00MsuR3jO6AzUwzK2Z6MIV3Tf4EDdITiuk8Uaz8K7g0R082WqsqNaSU8DdgkrRvZThzhit83mi/uiw==`; the file's SHA-256 is `37bea5005aa6d2dde929032ced5d2958cf642667cbc1fe675f9dfacf4ba15a51`). It uses no network, no `eval`, and the DOM only when one exists (it doesn't here). `pele/pele.d.ts` is ours: the part of its API the mod uses.
