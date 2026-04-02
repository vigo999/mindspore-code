package components

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// helper: create a TextInput, optionally set width, optionally call View()
// before returning (to populate viewport with stale content, simulating a
// prior render frame).
func newTestInput(width int, preRender bool) TextInput {
	ti := NewTextInput()
	ti = ti.SetWidth(width)
	if preRender {
		_ = ti.View()
	}
	return ti
}

// helper: paste into a TextInput, then re-set width (simulating
// resizeActiveLayout → resizeInput that the app does after every key).
func doPaste(t *testing.T, ti TextInput, text string, width int) TextInput {
	t.Helper()
	ti, _ = ti.Update(tea.KeyMsg{
		Type: tea.KeyRunes, Runes: []rune(text), Paste: true,
	})
	ti = ti.SetWidth(width)
	return ti
}

// helper: assert that every needle is present in the View output.
func assertViewContains(t *testing.T, ti TextInput, needles ...string) {
	t.Helper()
	view := ti.View()
	for _, n := range needles {
		if !strings.Contains(view, n) {
			t.Errorf("expected %q in view, got:\n%s", n, view)
		}
	}
}

// helper: type a rune-key event.
func typeRunes(ti TextInput, s string) (TextInput, tea.Cmd) {
	return ti.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)})
}

// helper: press a special key.
func pressKey(ti TextInput, k tea.KeyType) (TextInput, tea.Cmd) {
	return ti.Update(tea.KeyMsg{Type: k})
}

// ─── Scenario 1: Empty input, paste 2 short lines ──────────────────────
func TestScenario01_Paste2Lines(t *testing.T) {
	ti := newTestInput(80, true)
	ti = doPaste(t, ti, "The first line.\nThe second line.", 80)
	assertViewContains(t, ti, "first line", "second line")
}

// ─── Scenario 2: Empty input, paste 4 short lines ──────────────────────
func TestScenario02_Paste4Lines(t *testing.T) {
	ti := newTestInput(80, true)
	ti = doPaste(t, ti, "The first line.\nThe second line.\nThe third line.\nThe fourth line.", 80)
	assertViewContains(t, ti, "first line", "second line", "third line", "fourth line")
}

// ─── Scenario 3: Paste 4 lines, press ↑ repeatedly ─────────────────────
func TestScenario03_PasteThenCursorUp(t *testing.T) {
	ti := newTestInput(80, true)
	ti = doPaste(t, ti, "The first line.\nThe second line.\nThe third line.\nThe fourth line.", 80)

	for i := 0; i < 4; i++ {
		ti, _ = pressKey(ti, tea.KeyUp)
		ti = ti.SetWidth(80)
		assertViewContains(t, ti, "first line", "fourth line")
	}
}

// ─── Scenario 4: Paste 4 lines, press ↓ repeatedly ─────────────────────
func TestScenario04_PasteThenCursorDown(t *testing.T) {
	ti := newTestInput(80, true)
	ti = doPaste(t, ti, "The first line.\nThe second line.\nThe third line.\nThe fourth line.", 80)

	// cursor is at end already; pressing down should not scroll away first line
	for i := 0; i < 4; i++ {
		ti, _ = pressKey(ti, tea.KeyDown)
		ti = ti.SetWidth(80)
		assertViewContains(t, ti, "first line", "fourth line")
	}
}

// ─── Scenario 5: Paste 4 lines, submit ─────────────────────────────────
func TestScenario05_PasteThenSubmit(t *testing.T) {
	ti := newTestInput(80, true)
	ti = doPaste(t, ti, "The first line.\nThe second line.\nThe third line.\nThe fourth line.", 80)

	val := ti.Value()
	if !strings.Contains(val, "The first line.") || !strings.Contains(val, "The fourth line.") {
		t.Fatalf("submitted value incomplete: %q", val)
	}
}

// ─── Scenario 6: Lines of different lengths ─────────────────────────────
func TestScenario06_DifferentLengths(t *testing.T) {
	ti := newTestInput(80, true)
	ti = doPaste(t, ti, "a\nshort line\nthis is a longer line for testing\nend", 80)
	assertViewContains(t, ti, "a", "short line", "longer line", "end")
}

// ─── Scenario 7: Text with empty lines ──────────────────────────────────
func TestScenario07_EmptyLines(t *testing.T) {
	ti := newTestInput(80, true)
	ti = doPaste(t, ti, "line 1\n\nline 3\n\nline 5", 80)
	assertViewContains(t, ti, "line 1", "line 3", "line 5")
	if got := ti.Value(); got != "line 1\n\nline 3\n\nline 5" {
		t.Fatalf("empty lines not preserved: %q", got)
	}
}

// ─── Scenario 8: Trailing newline ───────────────────────────────────────
func TestScenario08_TrailingNewline(t *testing.T) {
	ti := newTestInput(80, true)
	ti = doPaste(t, ti, "line 1\nline 2\nline 3\n", 80)
	assertViewContains(t, ti, "line 1", "line 2", "line 3")
}

// ─── Scenario 9: Chinese multi-line ─────────────────────────────────────
func TestScenario09_Chinese(t *testing.T) {
	ti := newTestInput(80, true)
	ti = doPaste(t, ti, "第一行\n第二行\n第三行\n第四行", 80)
	assertViewContains(t, ti, "第一行", "第二行", "第三行", "第四行")
}

// ─── Scenario 10: Mixed Chinese/English ─────────────────────────────────
func TestScenario10_MixedChinEng(t *testing.T) {
	ti := newTestInput(80, true)
	ti = doPaste(t, ti, "first line\n第二行\nthird 行\n第四 line", 80)
	assertViewContains(t, ti, "first line", "第二行", "third", "第四")
}

// ─── Scenario 11: Narrow width causes soft wrap ─────────────────────────
func TestScenario11_SoftWrap(t *testing.T) {
	// Width 15 chars → prompt "❯ "(2) leaves 13 for text; each line wraps.
	ti := newTestInput(15, true)
	ti = doPaste(t, ti, "abcdefghijklmnop\nqrstuvwxyz12345", 15)
	assertViewContains(t, ti, "abcde", "qrstu")
}

// ─── Scenario 12: Existing long text + paste more ───────────────────────
func TestScenario12_AppendAfterLong(t *testing.T) {
	ti := newTestInput(80, false)
	// type some text first
	ti, _ = typeRunes(ti, "existing line")
	ti, _ = pressKey(ti, tea.KeyCtrlJ) // newline
	ti, _ = typeRunes(ti, "second existing")
	_ = ti.View() // render frame

	// now paste
	ti = doPaste(t, ti, "pasted line A\npasted line B", 80)
	assertViewContains(t, ti, "existing line", "second existing", "pasted line A", "pasted line B")
}

// ─── Scenario 13: Paste, then resize terminal ───────────────────────────
func TestScenario13_ResizeAfterPaste(t *testing.T) {
	ti := newTestInput(80, true)
	ti = doPaste(t, ti, "alpha\nbeta\ngamma\ndelta", 80)
	assertViewContains(t, ti, "alpha", "delta")

	// shrink
	ti = ti.SetWidth(30)
	assertViewContains(t, ti, "alpha", "delta")

	// expand
	ti = ti.SetWidth(120)
	assertViewContains(t, ti, "alpha", "delta")
}

// ─── Scenario 14: Paste then ctrl+j ────────────────────────────────────
func TestScenario14_PasteThenCtrlJ(t *testing.T) {
	ti := newTestInput(80, true)
	ti = doPaste(t, ti, "aaa\nbbb\nccc", 80)
	ti, _ = pressKey(ti, tea.KeyCtrlJ)
	ti = ti.SetWidth(80)
	assertViewContains(t, ti, "aaa", "bbb", "ccc")
}

// ─── Scenario 15: Paste then type in middle ─────────────────────────────
func TestScenario15_PasteThenEditMiddle(t *testing.T) {
	ti := newTestInput(80, true)
	ti = doPaste(t, ti, "aaa\nbbb\nccc", 80)
	// move cursor up into the middle
	ti, _ = pressKey(ti, tea.KeyUp)
	ti, _ = typeRunes(ti, "X")
	ti = ti.SetWidth(80)
	assertViewContains(t, ti, "aaa", "bbb", "ccc")
}

// ─── Scenario 16: Paste then backspace ──────────────────────────────────
func TestScenario16_PasteThenBackspace(t *testing.T) {
	ti := newTestInput(80, true)
	ti = doPaste(t, ti, "aaa\nbbb\nccc", 80)
	ti, _ = ti.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	ti = ti.SetWidth(80)
	assertViewContains(t, ti, "aaa", "bbb")
}

// ─── Scenario 17: Delete lines until 1 left ────────────────────────────
func TestScenario17_DeleteToOneLine(t *testing.T) {
	ti := newTestInput(80, true)
	ti = doPaste(t, ti, "aaa\nbbb", 80)

	// delete backwards: remove 'b','b','b','\n'
	for i := 0; i < 4; i++ {
		ti, _ = ti.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	}
	ti = ti.SetWidth(80)
	if h := ti.Height(); h > 4 {
		t.Errorf("height should shrink, got %d", h)
	}
	assertViewContains(t, ti, "aaa")
}

// ─── Scenario 18-19: History after paste ────────────────────────────────
func TestScenario18_HistoryAfterPaste(t *testing.T) {
	ti := newTestInput(80, false)
	ti = ti.PushHistory("single line")
	ti = ti.PushHistory("multi\nline\npaste")

	// recall most recent
	ti = ti.PrevHistory()
	if got := ti.Value(); got != "multi\nline\npaste" {
		t.Fatalf("expected multi-line history entry, got %q", got)
	}
	assertViewContains(t, ti, "multi", "line", "paste")

	// recall older
	ti = ti.PrevHistory()
	if got := ti.Value(); got != "single line" {
		t.Fatalf("expected single-line history entry, got %q", got)
	}
}

// ─── Scenario 20: Multi-line draft, press ↑ ─────────────────────────────
func TestScenario20_DraftUpInEditor(t *testing.T) {
	ti := newTestInput(80, false)
	ti.Model.SetValue("line X\nline Y")
	ti.syncHeight()

	// cursor is at end (line Y). Pressing up should move INSIDE the editor.
	if ti.CanNavigateHistory("up") {
		t.Fatal("should NOT trigger history while cursor is on the last line of multi-line input")
	}
}

// ─── Scenario 21: History + draft restore ───────────────────────────────
func TestScenario21_HistoryDraftRestore(t *testing.T) {
	ti := newTestInput(80, false)
	ti = ti.PushHistory("old entry")
	ti.Model.SetValue("my draft\nline 2")
	ti.syncHeight()

	ti = ti.PrevHistory()
	if got := ti.Value(); got != "old entry" {
		t.Fatalf("expected history, got %q", got)
	}
	ti = ti.NextHistory()
	if got := ti.Value(); got != "my draft\nline 2" {
		t.Fatalf("expected draft restored, got %q", got)
	}
}

// ─── Scenario 22: Slash → esc → paste ───────────────────────────────────
func TestScenario22_SlashEscPaste(t *testing.T) {
	ti := newTestInput(80, false)
	ti, _ = typeRunes(ti, "/")  // trigger slash mode
	ti, _ = pressKey(ti, tea.KeyEscape) // cancel

	ti = doPaste(t, ti, "aaa\nbbb\nccc", 80)
	assertViewContains(t, ti, "aaa", "bbb", "ccc")
}

// ─── Scenario 25: 8-15 lines ───────────────────────────────────────────
func TestScenario25_MediumPaste(t *testing.T) {
	var lines []string
	for i := 1; i <= 12; i++ {
		lines = append(lines, fmt.Sprintf("line %02d", i))
	}
	ti := newTestInput(80, true)
	ti = doPaste(t, ti, strings.Join(lines, "\n"), 80)
	assertViewContains(t, ti, "line 01", "line 12")
}

// ─── Scenario 26: Very long paste with max-height cap ──────────────────
func TestScenario26_VeryLongPaste(t *testing.T) {
	var lines []string
	for i := 0; i < 200; i++ {
		lines = append(lines, fmt.Sprintf("row %03d content here", i))
	}
	payload := strings.Join(lines, "\n")

	ti := newTestInput(80, true)
	ti = ti.SetMaxVisibleRows(20) // simulate a 24-line terminal (24-4=20)
	ti = doPaste(t, ti, payload, 80)

	if got := ti.Value(); got != payload {
		t.Fatalf("value mismatch: len %d vs %d", len(got), len(payload))
	}

	// With max height cap, the last line (where cursor is) must be visible.
	view := ti.View()
	if !strings.Contains(view, "row 199") {
		t.Fatalf("last line (cursor) not visible in view:\n%s", view)
	}

	// The height must be capped, not 200.
	if h := ti.editorHeight(); h > 20 {
		t.Errorf("editor height = %d, want ≤ 20", h)
	}
}

// ─── Scenario 26b: Long paste cursor stays visible during ↑/↓ ─────────
func TestScenario26b_LongPasteCursorNavigation(t *testing.T) {
	var lines []string
	for i := 0; i < 50; i++ {
		lines = append(lines, fmt.Sprintf("row %03d", i))
	}
	payload := strings.Join(lines, "\n")

	ti := newTestInput(80, false)
	ti = ti.SetMaxVisibleRows(10)
	_ = ti.View()
	ti = doPaste(t, ti, payload, 80)

	// Cursor should be at last row, visible.
	view := ti.View()
	if !strings.Contains(view, "row 049") {
		t.Fatalf("last row not visible after paste:\n%s", view)
	}

	// Press ↑ several times: cursor should move up, scroll follows.
	for i := 0; i < 5; i++ {
		ti, _ = pressKey(ti, tea.KeyUp)
		ti = ti.SetWidth(80)
	}
	view = ti.View()
	// After moving up 5 rows from 49, row 044 should be visible.
	if !strings.Contains(view, "row 044") {
		t.Errorf("row 044 not visible after pressing up 5 times:\n%s", view)
	}

	// Press ↓ back: should scroll down, last lines visible again.
	for i := 0; i < 5; i++ {
		ti, _ = pressKey(ti, tea.KeyDown)
		ti = ti.SetWidth(80)
	}
	view = ti.View()
	if !strings.Contains(view, "row 049") {
		t.Errorf("row 049 not visible after pressing down:\n%s", view)
	}
}

// ─── Scenario 26c: History recall of long entry ────────────────────────
func TestScenario26c_HistoryRecallLong(t *testing.T) {
	var lines []string
	for i := 0; i < 50; i++ {
		lines = append(lines, fmt.Sprintf("hist %03d", i))
	}
	longEntry := strings.Join(lines, "\n")

	ti := newTestInput(80, false)
	ti = ti.SetMaxVisibleRows(10)
	ti = ti.PushHistory(longEntry)
	ti = ti.PushHistory("short")

	// Recall "short" first.
	ti = ti.PrevHistory()
	if got := ti.Value(); got != "short" {
		t.Fatalf("expected 'short', got %q", got)
	}

	// Recall long entry.
	ti = ti.PrevHistory()
	if got := ti.Value(); got != longEntry {
		t.Fatalf("expected long entry, got len=%d", len(got))
	}

	// Last line should be visible (cursor at end after SetValue).
	view := ti.View()
	if !strings.Contains(view, "hist 049") {
		t.Fatalf("last line of recalled history not visible:\n%s", view)
	}
	if h := ti.editorHeight(); h > 10 {
		t.Errorf("editor height = %d, want ≤ 10", h)
	}
}

// ─── Scenario 27: 1 existing line + paste at end ───────────────────────
func TestScenario27_AppendToSingleLine(t *testing.T) {
	ti := newTestInput(80, false)
	ti, _ = typeRunes(ti, "hello ")
	_ = ti.View()

	ti = doPaste(t, ti, "world\nfoo\nbar", 80)
	assertViewContains(t, ti, "hello", "world", "foo", "bar")
}

// ─── Scenario 28: Paste in the MIDDLE of existing multi-line text ──────
func TestScenario28_PasteInMiddle(t *testing.T) {
	ti := newTestInput(80, false)

	// Set up "before\nafter" and position cursor at end of "before".
	ti.Model.SetValue("before\nafter")
	ti.Model.CursorUp()  // row 0
	ti.Model.CursorEnd() // col 6 = end of "before"
	ti.syncHeight()
	_ = ti.View()

	// Paste "X\nY" at end of "before".
	// Expected result: "beforeX\nY\nafter", cursor on the "Y" row (row 1).
	ti = doPaste(t, ti, "X\nY", 80)

	val := ti.Value()
	if val != "beforeX\nY\nafter" {
		t.Fatalf("unexpected value: %q", val)
	}

	// All content must be visible.
	assertViewContains(t, ti, "beforeX", "Y", "after")

	// Cursor should be on the "Y" row (row 1), NOT on "after" (row 2).
	if row := ti.Model.Line(); row != 1 {
		t.Errorf("cursor row = %d, want 1 (the Y row)", row)
	}
}
