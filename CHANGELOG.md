# Changelog

Each release names the version of terminu design (tdp) it follows.

## Unreleased

- First version: `viewu <path>` shows a file or a directory in one panel, the statusbar over it with its size, when it
  was modified and its mode; `viewu version` and `viewu help`
- A text file in the colours of its language, as the file preview colours it, bold and italic included, with line
  numbers; long lines fold onto the rows under them. What chroma leaves uncoloured is filled as the family fills it: a
  TOML file's tables and keys, a YAML value without quotes, and more. A name several formats share is told by its
  folder: a `config` in `.ssh` or `.git`, a `hosts` in `/etc`
- A JSON file as it is, and formatted in its `FORMATTED` tab
- A Markdown file drawn as a document, as webu draws a page: headings on the colour of their level, lists, code blocks
  in their language's colours, quotes, tables as grids, links and images with their icons; its source in the `SOURCE`
  tab
- Mermaid diagrams drawn in the terminal, laid out by Pele 1.13.0 to fit the panel: a `mermaid` block in Markdown, and a
  `.mmd` or `.mermaid` file with its `DIAGRAM` and `SOURCE` tabs. One that cannot be drawn shows its source with the
  reason under it, and a warning in the App Log
- A directory or an archive as a tree, a PDF's text, an image as a data URI, a binary in hex
- `j|k`, `u|d`, `gg|G` scroll; `v` selects with vim's moves and `y` copies; `/` finds text and `n|N` go through it; `o`
  opens another file
- The settings are the family's `input_tab_width` and `hide_secrets_in_preview`, in `~/.config/viewu/config.yaml`
- Built on terminu-go v1.2.0-beta.2, following tdp v1.2.0-beta.2
