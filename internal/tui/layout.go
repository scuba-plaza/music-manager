package tui

import (
	"os"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// layoutMode says how the screen is divided.
type layoutMode int

const (
	modeTooSmall layoutMode = iota // not enough room for anything useful
	modeOnePane                    // one pane at a time
	modeTwoPane                    // browser and inspector side by side
)

const (
	minWidth  = 60
	minHeight = 16
)

// layoutInfo is how a screen of a given size is divided. The header and the
// footer take one line each; body is the height of the panes.
type layoutInfo struct {
	mode        layoutMode
	left, right int // pane widths, borders included
	body        int
}

func computeLayout(w, h int) layoutInfo {
	if w < minWidth || h < minHeight {
		return layoutInfo{mode: modeTooSmall}
	}
	l := layoutInfo{body: h - 2}
	if w < 70 {
		l.mode, l.left, l.right = modeOnePane, w, w
		return l
	}
	l.mode = modeTwoPane
	if w >= 100 {
		l.left = clamp(w*40/100, 34, 56)
	} else {
		l.left = clamp(w*38/100, 28, 40)
	}
	l.right = w - l.left
	return l
}

func clamp(v, lo, hi int) int { return max(lo, min(hi, v)) }

// box draws content inside a rounded border with title set into the top
// edge. The result is exactly w columns by h lines: longer lines are cut
// short and missing lines are blank.
func box(title string, content []string, w, h int, focused bool) string {
	if w < 4 || h < 2 {
		return strings.TrimRight(strings.Repeat(strings.Repeat(" ", max(w, 0))+"\n", max(h, 0)), "\n")
	}
	border, titleStyle := styles.border, styles.borderTitle
	if focused {
		border, titleStyle = styles.borderFocus, styles.borderTitleFocus
	}
	inner := w - 2

	top := ""
	if title != "" && inner >= 6 {
		t := ansi.Truncate(title, inner-4, "…")
		top = " " + titleStyle.Render(t) + " "
	}
	rest := inner - ansi.StringWidth(top)
	lines := make([]string, 0, h)
	lines = append(lines, border.Render("╭")+top+border.Render(strings.Repeat("─", rest)+"╮"))
	side := border.Render("│")
	for i := 0; i < h-2; i++ {
		line := ""
		if i < len(content) {
			line = content[i]
		}
		lines = append(lines, side+fit(line, inner)+side)
	}
	lines = append(lines, border.Render("╰"+strings.Repeat("─", inner)+"╯"))
	return strings.Join(lines, "\n")
}

// fit makes s exactly w columns wide: cut with an ellipsis if too long, padded
// with spaces if too short. It understands colour codes and wide characters.
func fit(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if ansi.StringWidth(s) > w {
		s = ansi.Truncate(s, w, "…")
		if strings.Contains(s, "\x1b") {
			s += "\x1b[m" // a cut can leave a colour open
		}
	}
	if pad := w - ansi.StringWidth(s); pad > 0 {
		s += strings.Repeat(" ", pad)
	}
	return s
}

// spread puts left at the left edge and right at the right edge of a line w
// columns wide. When they do not both fit, the left side gives way.
func spread(left, right string, w int) string {
	rw := ansi.StringWidth(right)
	if rw >= w {
		return fit(right, w)
	}
	if lw := ansi.StringWidth(left); lw+rw+1 > w {
		left = ansi.Truncate(left, max(w-rw-1, 0), "…")
	}
	gap := w - ansi.StringWidth(left) - rw
	return left + strings.Repeat(" ", gap) + right
}

// tilde writes a path under the home directory with a leading ~.
func tilde(p string) string {
	home := os.Getenv("HOME")
	switch {
	case home == "":
		return p
	case p == home:
		return "~"
	case strings.HasPrefix(p, home+"/"):
		return "~" + strings.TrimPrefix(p, home)
	}
	return p
}
