# catu

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

- TOML 的 key 與表名也要有顏色：catu 自己補（使用者選的）。
- JSON 多一個排過版（pretty print）的 tab，預設還是看原檔。
- mermaid 的菱形（與其他斜的線）用階梯狀的框線畫，不要一堆點。

## 2026-11-03（使用者發現的 bug，terminu 的 session 轉來）

使用者從 Space menu 執行 `/`，打字按 `Enter` 之後 menu 還蓋在文字上，看不到找到的字；要按 `Esc` 關掉 menu 才看得到。

- 從 Space menu 搜尋，真的搜了就連 menu 一起收起，有沒有找到都一樣。
- 在搜尋的 input popup 上按 `Esc` 回到 menu。

## 2026-11-04（改名）

> 新名字 "catu" 就是 cat 指令  + u

- app 從 viewu 改名成 catu（`cat` 指令加 u）：指令、設定目錄、環境變數、splash 與 icon 裡的字都跟著改。

## 2026-11-04（判斷的菱形）

> 我覺得 mermaid 那個「判斷」的菱形呈現方式很糟糕，我們換掉吧

- 流程圖的判斷（`{…}`）不再畫成菱形，換成在終端機裡好看的樣子（取代 2026-11-03 的「菱形用階梯狀的框線畫」）。

## 2026-11-04（不要點點）

> 一樣，不要有點點的那種表現方式

- mermaid 圖裡不要有點點（點字）畫的東西：彎的線、圓都改用框線畫。
- 圓餅圖用框線畫不出圓：改畫成橫條（使用者從兩個選項選的：橫條，或圓餅圖照舊用點點）。

## 2026-11-04（圓餅圖只有一條 bar）

> pie 圖不應該是那樣呈現，應該是只有一個 bar, 然後用不同顏色，然後下方標示每種顏色的 % 數和代表，呈現上應該是
> bar
> <color1> <%數> <color2> <%數> ...
> <color1> <描述>
> <color2> <描述>

- 圓餅圖畫成一條 bar，每一塊用不同的顏色。
- bar 下面一列是每個顏色與它的百分比，再來一個顏色一列寫它代表什麼。

## 2026-11-04（命令列）

> 我想要在 cli 的部分多加一點東西，但想法很模糊，例如可以串接 pipeline?

- 使用者從四種用法裡全選：
  - 從管線讀進來看：`git log | catu`、`curl -s 網址 | catu --lang md`，像 less。
  - 輸出接到管線或檔案時不開畫面，像 cat 一樣印出來；Markdown、mermaid 印成畫好的文字。
  - 加參數直接在終端機印出來、不開畫面：`catu -p a.md`，像 bat -p。
  - 一次看好幾個檔：`catu a.go b.go`。

## 2026-11-16（檔案改了就重讀）

> 我想要加 watch 檔案更新即時 reload 功能

- 檔案在磁碟上變了，catu 自動重讀、畫面跟著更新。
- 一直開著，不用加參數（使用者在「一直開著」與「加 -w/--watch 才開」之間選了前者）。

## 2026-11-16（META 分頁、框上的 sync icon）

> 你的顯示方式直接佔用了一 row，我們應該多一個 META 分頁來放檔案資訊，然後在 border 找一個地方顯示 nf-md-sync 來顯示持續 watch 中

> 我希望那個 sync icon 放在右下角，然後配上 sync 時間

- 最上面那一列（大小、修改時間、權限）拿掉，檔案的資訊改放在一個 META 分頁。
- 框上顯示 nf-md-sync，表示正在 watch；放在右下角，旁邊寫 sync 的時間。

## 2026-11-16（Visual 中也重讀）

> 然後 visual mode 是不能夠寫入只能複製的，所以在 visual mode 的時候應該也是要 watch reload 的

- Visual 模式只能選取、複製，不會寫入；在 Visual 模式中檔案改了，也照常重讀。

## 2026-11-16（Visual 中重讀時說一聲）

> 1. visual 中內容變了，重新刷新畫面，跳一個 info, 說內容變了

- 在 Visual 模式中檔案改了：照樣重讀、更新畫面，並跳一個 Info toast 說內容變了。

## 2026-11-16（Visual 中不跳 toast）

> 不然也不需要跳 toast, 反正有 reload icon 和時間，可以看到那個時間一直在更新

- Visual 中重讀不跳 toast：框上的 sync icon 與時間一直在更新，看得到重讀了。

## 2026-11-16（Visual 中重讀時取消選取）

> visual mode 中，畫面刷新，如果有 select 的情況，把 select 區域移除取消, 游標停留在原本的行列號, 但是照樣刷新

- Visual 模式中重讀：有選取的話，取消選取；游標停在原本的行與列；畫面照常刷新。

## 2026-11-16（sync icon 有了位置）

> tdp/tgo session 都開了, 我希望那個 sync icon 放在右下角，然後配上 sync 時間

- tdp v1.2.12 的 panel.1 讓 panel 的框上放一個 app 自己的持續狀態，terminu-go v1.2.13 的 Panel.FrameStatus 畫它：sync icon 放在右下角，旁邊寫最後一次讀進來的時間。

## 2026-11-17（watch 剩下的幾點）

> 1. 既然這樣那就不要取消，有選舉"範圍"和 cursor 位置保留，但是內容不管，內容有可能不同
> 2. 一般狀態下跟著往下，visual mode 不跟著往下
> 3. 這是 tdp 沒考慮到檔案刷新，這應該要請 tdp 放
> 4. catu 不管「是不是 log」，只管「是哪種檔案格式」，所以 catu 會變暗，但沒有違反，因為在 catu 中都是檔案，而不是 log, log 檔頂多也只是一種檔案格式，和 json 一樣(在 catu 中)
> 5. 用 Error
> 7. by (1) 選取不取消, by (2) visual 不往下，所以就沒這問題了吧

- Visual 中重讀不取消選取：選取範圍與游標的位置留著，範圍裡的內容不管，可能已經不一樣。
- 停在最底下時，一般狀態跟著新內容往下；Visual 中不跟。
- 重讀後行號欄的寬度：tdp 沒考慮到檔案會重讀，請 tdp 補。
- catu 不分是不是 log，只分檔案格式；log 檔在 catu 裡跟 JSON 一樣只是一種格式，失焦時照樣變暗，不違反 tdp。
- 重讀時讀不了用 Error。

## 2026-11-17（tdp note.9：開著時會變的內容）

使用者在 terminu session 把 watch 剩下的幾點定進 tdp note.9（terminu 本機 commit 01837596，還沒發版；terminu session 轉來）：

- 重讀等於重新打開。選取模式只保留游標與選取起點的「位置」，不保留字：兩個都在什麼都不變；起點不在就拿掉選取、回到 Visual、跳 Warning；游標不在就離開模式、跳 Warning；整個讀不到時離開模式，只跳讀不到的 Error。不收到最後一個字。
- 跟著結尾：不在選取模式時跟著往下，往上捲過的停在原本那幾行；選取模式一律不跟；內容變得比畫面短時畫面移到新的結尾。
- 行號欄開著的時候只變寬，換別的檔或重新打開才重算。
- 搜尋在新內容裡重新找：目前那一筆是原本位置或之後的第一筆，畫面不動；一筆都不剩就是一次沒找到的搜尋（Info toast）。開著的 Space menu 與 ? 不為此改變。
- 不算串流內容，失焦時變暗，log 檔也一樣。

## 2026-11-17（選單開著時選取失效，tdp v1.2.14）

使用者在 terminu session 定進 tdp v1.2.14 note.9（catu 的 review 問的，使用者「送給 terminu，請 tdp 定」）：

- 選取模式的 Space menu 或 ? 開著時，重讀讓選取失效（起點或游標不見）：不跳 Warning toast，改在它們上面疊一個 Peach、跟 error popup 一樣的 note 說明。
- 它們開著時檔案讀不到：不跳 Error toast，改疊 error popup，說讀不到、選取失效了。
- 兩種都按 Enter 或 Esc 連同底下的 menu 或 ? 一起關掉，回到畫面。

