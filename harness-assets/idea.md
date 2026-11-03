# viewu

## 2026-11-03

> 作一個看檔案的工具，就是把 filu 那種檔案預覽，和各種檔案的 syntaxhighlight, 做成一個簡易看檔案的工具，就像 cat,bat
> 那樣，但我真的想要做的是類似 leaf, 但比 leaf 強, 我想使用 https://github.com/obsidianmd/pele 來出 mermaid 圖, 一樣有 vim motion 來控制 cursor, 有 view
> mode 和 edit mode, 預設 view mode,

- 一個看檔案的工具：filu 那種檔案預覽，各種檔案都有 syntax highlight；簡易，像 cat、bat 那樣。
- 真正想做的是類似 leaf（RivoLink/leaf，終端機裡的 Markdown previewer），但比 leaf 強。leaf 只是 idea 的起點，不照 leaf 那套。
- Markdown 照 webu 顯示 markdown 的方式。
- 用 pele（https://github.com/obsidianmd/pele）出 mermaid 圖。
- 用 vim motion 控制 cursor。
- 先不要有 edit mode，單純看檔案就好。

## 2026-11-03（看過 samples 之後）

> md 檔
> 1. head 6 也給一個獨立顏色，不要和 head 1 一樣
> 2. ouotes in quotes 的那個直槓，也 follow header 的顏色層級

- Markdown 的第 6 層標題有自己的顏色，不要跟第 1 層一樣。
- 引用裡的引用，那條直槓也照標題的顏色層級。

## 2026-11-03（看過 samples 之後，第二次）

> toml 的 syntaxhighlight 好像沒有？json 可以展開成 prettyprint 嗎? 還是保持原檔案形式真實呈現？那個流程圖的菱形用一堆點構成是 pele 的作法？

- TOML 的 key 與表名也要有顏色：viewu 自己補（使用者選的）。
- JSON 多一個排過版（pretty print）的 tab，預設還是看原檔。
- mermaid 的菱形（與其他斜的線）用階梯狀的框線畫，不要一堆點。
