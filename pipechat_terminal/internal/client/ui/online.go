package ui

import (
	"fmt"
	"strings"
)

// OnlinePanel is the list under the chats in the sidebar: who is here right now. handles are the other
// people the server says have the chat open; you is your own handle, shown first when you are connected.
// room is how many lines it may use.
func OnlinePanel(handles []string, you string, connected bool, room int) string {
	if room < 3 {
		return ""
	}
	// You are online by definition; the server's list can lag behind for a few seconds.
	if you != "" && connected && !contains(handles, you) {
		handles = append([]string{you}, handles...)
	}
	lines := []string{Dim.Render(strings.Repeat("─", SidebarWidth-2)), Active.Render(fmt.Sprintf("Online (%d)", len(handles)))}
	for index, handle := range handles {
		if len(lines) >= room {
			lines = append(lines, Dim.Render(fmt.Sprintf("  +%d more", len(handles)-index)))
			break
		}
		label := "@" + handle
		if handle == you {
			label += " (you)"
		}
		lines = append(lines, OnlineDot+" "+Truncate(label, SidebarWidth-5))
	}
	return strings.Join(lines, "\n")
}

func contains(list []string, item string) bool {
	for _, entry := range list {
		if entry == item {
			return true
		}
	}
	return false
}
