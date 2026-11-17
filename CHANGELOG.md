# Changelog

Each release names the version of terminu design (tdp) it follows.

## v1.1.1 — tdp v1.2.14

- With the Visual mode's `Space` menu or `?` open, a read that takes the selection away says so in a note over them,
  not in a toast: in Peach when the selection or the mode had to go, as an error when the file cannot be read; `Enter`
  or `Esc` closes it and them together
- Built on terminu-go v1.2.15, following tdp v1.2.14

## v1.1.0 — tdp v1.2.13

- The file shown is watched: changed on the disk, it is read again within a second, with no key pressed and nothing to
  turn on. The view stays on the line it was on, or, at the end, follows what was added, as `tail -f`; a search is found
  again; the tab stays. In the Visual mode too: where the cursor and the selection's start still are, they stay, over
  the text as it is now; where they are gone, the selection or the mode goes, said in a warning; the view does not
  follow what is added. The number column only widens; a search with nothing left says so. The panel's bottom right says
  when the file was last read (`󰓦 17:43:42`), in Red while it is gone or cannot be read; a file gone stays shown as it
  was, said once. What came down a pipe is not watched
- A `META` tab, the last of every file: its type, size, when it was modified and its mode, as the file preview writes
  them. The row over the panel that said them is gone, and the panel has it
- A search's status on the panel's bottom border while it has hits: what was searched for, which hit and how many
  (`/needle 2 of 5`)
- Built on terminu-go v1.2.14, following tdp v1.2.13

## v1.0.0 — tdp v1.2.0

- First version: `catu <path>` shows a file or a directory in one panel, the statusbar over it with its size, when it
  was modified and its mode; `catu version` and `catu help`
- Printed instead of shown when the output is not a terminal, as `cat` prints: a text file byte for byte, a Markdown file
  or a mermaid diagram drawn `$COLUMNS` wide, with no colours; `catu -p <path>` prints it on the terminal in colour and
  quits
- What comes down a pipe, as `git log | catu` or `catu -`: shown, or printed; `-l md`, `-l mermaid`, `-l go` and so on
  name its format
- `catu a.go b.md …`: the files listed in a FILES panel, the one under its cursor shown beside it; printed, one after
  another as `cat a b` does
- A text file in the colours of its language, as the file preview colours it, bold and italic included, with line
  numbers; long lines fold onto the rows under them. What chroma leaves uncoloured is filled as the family fills it: a
  TOML file's tables and keys, a YAML value without quotes, and more. A name several formats share is told by its
  folder: a `config` in `.ssh` or `.git`, a `hosts` in `/etc`
- A JSON file as it is, and formatted in its `FORMATTED` tab
- A Markdown file drawn as a document, as webu draws a page: headings on the colour of their level, lists, code blocks
  in their language's colours, quotes, tables as grids, links and images with their icons; its source in the `SOURCE`
  tab
- Mermaid diagrams drawn in the terminal, laid out by Pele 1.13.0 to fit the panel: a `mermaid` block in Markdown, and a
  `.mmd` or `.mermaid` file with its `DIAGRAM` and `SOURCE` tabs. A flowchart's decision is a box as tall as the others
  with a ◆ on either side, not a diamond. Nothing is drawn in dots: a curve turns at right angles, a circle is a box
  with `(` and `)` on its sides, a state diagram's end is ◉, and a pie chart is one bar in its slices' colours, the
  biggest first, each colour's share and name under it. One that cannot be drawn shows its source with the reason under
  it, and a warning in the App Log
- A directory or an archive as a tree, a PDF's text, an image as a data URI, a binary in hex
- `j|k`, `u|d`, `gg|G` scroll; `v` selects with vim's moves and `y` copies; `/` finds text and `n|N` go through it; `o`
  opens another file
- The settings are the family's `input_tab_width` and `hide_secrets_in_preview`, in `~/.config/catu/config.yaml`
- Built on terminu-go v1.2.0, following tdp v1.2.0

## v1.0.0-beta.1 — tdp v1.2.0

- First version: `catu <path>` shows a file or a directory in one panel, the statusbar over it with its size, when it
  was modified and its mode; `catu version` and `catu help`
- Printed instead of shown when the output is not a terminal, as `cat` prints: a text file byte for byte, a Markdown file
  or a mermaid diagram drawn `$COLUMNS` wide, with no colours; `catu -p <path>` prints it on the terminal in colour and
  quits
- What comes down a pipe, as `git log | catu` or `catu -`: shown, or printed; `-l md`, `-l mermaid`, `-l go` and so on
  name its format
- `catu a.go b.md …`: the files listed in a FILES panel, the one under its cursor shown beside it; printed, one after
  another as `cat a b` does
- A text file in the colours of its language, as the file preview colours it, bold and italic included, with line
  numbers; long lines fold onto the rows under them. What chroma leaves uncoloured is filled as the family fills it: a
  TOML file's tables and keys, a YAML value without quotes, and more. A name several formats share is told by its
  folder: a `config` in `.ssh` or `.git`, a `hosts` in `/etc`
- A JSON file as it is, and formatted in its `FORMATTED` tab
- A Markdown file drawn as a document, as webu draws a page: headings on the colour of their level, lists, code blocks
  in their language's colours, quotes, tables as grids, links and images with their icons; its source in the `SOURCE`
  tab
- Mermaid diagrams drawn in the terminal, laid out by Pele 1.13.0 to fit the panel: a `mermaid` block in Markdown, and a
  `.mmd` or `.mermaid` file with its `DIAGRAM` and `SOURCE` tabs. A flowchart's decision is a box as tall as the others
  with a ◆ on either side, not a diamond. Nothing is drawn in dots: a curve turns at right angles, a circle is a box
  with `(` and `)` on its sides, a state diagram's end is ◉, and a pie chart is one bar in its slices' colours, the
  biggest first, each colour's share and name under it. One that cannot be drawn shows its source with the reason under
  it, and a warning in the App Log
- A directory or an archive as a tree, a PDF's text, an image as a data URI, a binary in hex
- `j|k`, `u|d`, `gg|G` scroll; `v` selects with vim's moves and `y` copies; `/` finds text and `n|N` go through it; `o`
  opens another file
- The settings are the family's `input_tab_width` and `hide_secrets_in_preview`, in `~/.config/catu/config.yaml`
- Built on terminu-go v1.2.0, following tdp v1.2.0
