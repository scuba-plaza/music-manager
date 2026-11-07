# What the new page keeps of the old one

One line for each row of the key table and each paragraph of "The web interface" in `README.md`. Each line names the test that pins it. Test names are given without `test(` or `it(`; files are under `web/` (`e2e/` is Playwright against the seeded server in both the desktop and the narrow project, `src/...browser.test.tsx` runs in a real browser, `src/...test.ts` is unit).

NEW means the test was written for this audit (`e2e/parity.spec.ts`, `e2e/rtl.spec.ts`, `e2e/empty.spec.ts`).

## The key table

| README row | Pinned by |
|---|---|
| `↑↓` `/` move, filter | e2e/tree.spec.ts: the slash key focuses the filter, enter keeps it and escape clears it, and the footer follows; status:unidentified shows only the unidentified track; a filter that matches nothing says so; typing j, k and space in the filter box types them and marks nothing |
| `z` centre | src/state/actions.test.ts: centre asks the tree to scroll the cursor row to the middle |
| `S` sort | e2e/dialogs.spec.ts: S then 3 sorts by title into one flat list and choosing the tree goes back to the tree; e2e/rtl.spec.ts: sorting by artist puts فيروز among the F, by its Latin form (NEW) |
| `+` `-` | e2e/tree.spec.ts: + opens a level and - closes all of it |
| `enter` `←` open and close | e2e/tree.spec.ts: enter opens an artist and its albums, and again closes them; src/state/actions.test.ts: close closes an open row, and on a closed row goes up to its parent |
| `→` `e` into the fields, `enter` edits, saves, `esc` cancels, `←` back | e2e/inspector.spec.ts: editing the title saves it and the tree row follows; a refused title stays open with the reason under it; src/components/Inspector.browser.test.tsx: enter edits a field and escape cancels; src/state/edits.test.ts: with nothing marked, e goes into the fields of a track, an artist, an album or a playlist |
| `→` `←` `1` `2` `3` `4` tabs, the tab stays | src/components/Inspector.browser.test.tsx: the number keys and the arrows turn the tabs, and ← on the first goes back to the tree; e2e/inspector.spec.ts: the device tab previews the track with the profile chosen; e2e/parity.spec.ts: the tab you are on stays when another track is selected (NEW) |
| `[` `]` `{` `}` `0` `p` cuts | src/state/edit.test.ts: [ ] { } nudge the field by a step or a big step; p fetches the 8 s around the cut and plays it; e2e/parity.spec.ts: ] nudges a cut by a second, [ by a tenth, { and } by ten, and 0 clears it (NEW, against the real server) |
| `space` `ctrl+a` `e` marks and the bulk form | e2e/tree.spec.ts: space marks a track and an album marks all its tracks; ctrl+a marks every track that is shown; e2e/editing.spec.ts: marking two tracks and e sets the album on both with one undo step |
| `J` `K` `n` | e2e/editing.spec.ts: J moves a track down and renumbers; K on the first track of an album says it is the first already; e2e/parity.spec.ts: n numbers the album and the tracks follow (NEW) |
| `c` `v` `ctrl+v` | e2e/editing.spec.ts: c on a field of a track copies that field, and v pastes it on an album; pasting a copied cover; e2e/parity.spec.ts: a picture pasted with ctrl+v becomes the cover of the track under the cursor (NEW, a real paste event) |
| `p` playlists, `J` `K` `d` `→` in them | e2e/editing.spec.ts: p adds the selected album to Late night and the count becomes 3; p then n makes a new playlist with the track; J in a playlist moves the track down the playlist; d on a track of a playlist takes it out of the playlist and leaves it in the library; d on a playlist asks, and y deletes it; e2e/empty.spec.ts: a library without playlists makes p open on the name input (NEW) |
| `i` `I` `o` | e2e/identify.spec.ts: i on the unidentified track lists candidates, shows what would change, and applying identifies it; I identifies all with a progress bar; o adopts the file's old tags |
| `a`, dropped MP3 files | e2e/add.spec.ts: a adds from a link, pre-filled, and the path preview shows where it will be written; F2 ignores the match and uses what the download said; a duplicate is flagged; dropping two mp3 files adds two unidentified tracks |
| `s` sync, the Format row | e2e/sync.spec.ts: s opens the targets and the library folder has no format row; a typed folder offers every profile, generic first, and w goes through them; strays are removed only after typing delete |
| `d` `u` `ctrl+r` `D` | e2e/editing.spec.ts: d asks and y removes the track and it disappears from its playlists; n on the question removes nothing; u then ctrl+r undoes and redoes; e2e/identify.spec.ts: D lists the problems and a click moves the cursor to the track |

## The paragraphs

| Paragraph | Pinned by |
|---|---|
| What the right-hand pane shows for a track, an album, an artist, a playlist, the Needs work group | src/components/Inspector.browser.test.tsx: a track: its title, its fields, where its file goes, and its names in their own direction; an album: its fields, what is inside it, and the title of the pane; the Needs work group lists what each track lacks; e2e/empty.spec.ts: an unidentified track shows its fields and the Needs work group lists it (NEW) |
| Changes are saved at once; the header counts what waits to sync and the chip opens the sync | e2e/shell.spec.ts: the header shows the library folder, the tracks and what waits to sync; e2e/sync.spec.ts: the header says N to sync · press s and the chip opens the dialog |
| `?` lists the keys by page, `←` `→` `1`-`4` turn them, the bottom line shows the ones that matter | e2e/dialogs.spec.ts: the help dialog lists every binding by page and ← → turns the pages; src/keys/keymap.test.ts: the tree shows the keys of the old footer, in its order |
| Several programs work on one library; the page notices what the others did | src/api/events.test.ts: listens to the event stream of the server; e2e/parity.spec.ts: what another program changes shows on the page within a few seconds (NEW); the locking is the server's: internal/app tests |
| Serving on this computer only, one port, `--no-open`, plain `mm` | not the page: internal/launch tests (TestNoArgumentsStartsTheServerOnTheDefaultAddress, TestAddrAndNoOpenFlagsArePassedOn) and internal/web's host and X-MM checks |
| Covers are pictures; a dropped or pasted picture is the cover; the Cover field's enter chooses a file | e2e/editing.spec.ts: a picture dropped on the page becomes the cover of the track under the cursor; enter on the Cover field chooses a file, which becomes the cover; dropping a non-image on the page is refused with a message; e2e/parity.spec.ts: x on the Cover field removes the cover of the track (NEW) |
| The mouse: click selects, double-click opens, the arrow opens at once, click a field then again edits, drag the divider | e2e/tree.spec.ts: a double click opens a row and a click on the arrow opens another at once; e2e/shell.spec.ts: dragging the divider changes the left width and survives a reload; e2e/parity.spec.ts: click selects, double-click opens, click on a field then again edits, the twist arrow opens at once (NEW) |
| Dark and light theme, remembered | e2e/shell.spec.ts: the theme starts dark and the light one survives a reload |
| One pane below 800 pixels | e2e/shell.spec.ts: below 800px one pane shows at a time: → into the fields shows the right pane and ← returns to the tree; both panes show from 800px and one pane below it; no horizontal scrollbar at 500px |
| Right-to-left names are laid out in their own direction without reordering the text around them | e2e/tree.spec.ts: the Arabic artist row is a bdi with dir=auto; an Arabic name does not move its neighbours; e2e/rtl.spec.ts: Arabic rows keep dir=auto and the count column does not move; a mixed Latin and Arabic title does not reorder its neighbours; the Latin tab and the Device tab show the Latin form for the Arabic track (all NEW) |
| Empty cases | e2e/empty.spec.ts: an empty library shows an empty tree and keys do nothing harmful; an empty playlist opens with no rows and d deletes it after asking (NEW) |

## Gaps found

None of the lines needed a fix to the page: every NEW test passed on the first run against the real server once its own locators were right.
