package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// Box-drawing helpers shared by the list, detail, and diff views. Every view
// renders a full-width frame where innerWidth is the content width between
// the "│ " / " │" gutters.

// frameRow wraps one content line in the vertical frame borders, padding the
// content to innerWidth.
func frameRow(content string, innerWidth int) string {
	return fr("│ ") + padRight(content, innerWidth) + fr(" │")
}

// frameTop renders the top border with an unstyled header: ╭─ header ────╮
func frameTop(header string, innerWidth int) string {
	dashes := max(0, innerWidth-lipgloss.Width(header)-2)
	return fr("╭─ " + header + " " + strings.Repeat("─", dashes) + "╮")
}

// frameSectionHeader renders a divider with an embedded (possibly styled)
// title: ├─ Title ────┤
func frameSectionHeader(styledTitle string, innerWidth int) string {
	dashes := max(0, innerWidth-lipgloss.Width(styledTitle)-1)
	return fr("├─ ") + styledTitle + fr(" "+strings.Repeat("─", dashes)+"┤")
}

// frameDivider renders a plain horizontal divider: ├──────┤
func frameDivider(innerWidth int) string {
	return fr("├" + strings.Repeat("─", innerWidth+2) + "┤")
}

// frameBottom renders the bottom border: ╰──────╯
func frameBottom(innerWidth int) string {
	return fr("╰" + strings.Repeat("─", innerWidth+2) + "╯")
}
