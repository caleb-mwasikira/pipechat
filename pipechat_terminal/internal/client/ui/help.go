package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"pipechat/internal/client/files"
	"pipechat/internal/client/search"
)

// One help entry: what to type, and what it does.
type helpItem struct{ usage, what string }

type helpSection struct {
	title string
	items []helpItem
}

var helpSections = []helpSection{
	{"Groups", []helpItem{
		{"/create NAME", "make a new private group (1-24 characters)"},
		{"/create NAME -open", "make a public group anyone can join by name"},
		{"/join CODE|NAME", "join with an invite code, or a public group by name"},
		{"/public", "list the public groups (/unlist NAME makes yours private)"},
		{"/close · /closed · /reopen NAME", "take a chat out of the sidebar, list those, bring one back"},
		{"/code", "show this group's invite code; share it privately"},
		{"/invite EMAIL", "email the invite code to someone"},
		{"/groups", "list your groups"},
		{"/switch N", "go to group number N from /groups"},
		{"/kick HANDLE", "remove a member (only the group's creator can)"},
		{"/leave", "leave the current group"},
	}},
	{"Direct messages", []helpItem{
		{"/dm HANDLE", "start a private chat with someone"},
		{"/whois HANDLE", "open someone's profile: bio, status, key (/me is yours)"},
		{"/bio TEXT", "write your bio, up to 280 characters (/bio clear removes it)"},
	}},
	{"Files", []helpItem{
		{"/upload PATH [-name SHORT]", "send a file or folder, encrypted, to this chat"},
		{"/download SHORT|LINK [DIR]", "save a shared file"},
		{"/files", "list files shared with you"},
		{"/link SHORT", "show a file's download link (it holds the key)"},
		{"/share SHORT", "post a file you already have to this chat"},
	}},
	{"Music", []helpItem{
		{"/music", "list the shared music library (upload on the website)"},
		{"/play", "stream the whole library, one song after another"},
		{"/play NUMBER", "start from that track, then carry on"},
		{"/save NUMBER|all [DIR]", "keep songs from the library as files (default: your Music folder)"},
		{"/next · /stop", "skip to the next song · stop the music"},
		{"/shuffle [on|off]", "play in random order"},
		{"/repeat off|one|all", "repeat nothing, this song, or the whole library"},
	}},
	{"Search", []helpItem{
		{"/search TEXT", "find TEXT in every chat (also the groups you closed)"},
		{"/search -here TEXT", "only in the chat you are in"},
		{"/search -in CHAT TEXT", "only in one chat (quote a name with spaces)"},
	}},
	{"Screen", []helpItem{
		{"/clear", "clear this chat's messages from view"},
		{"/help", "show this guide"},
		{"/update", "check for a newer version and offer to install it"},
		{"/exit", "quit (/quit works too)"},
	}},
}

var helpKeys = []helpItem{
	{"Tab", "finish the command you are typing (grey text); also next group"},
	{"Tab · Ctrl+N", "next group"},
	{"Shift+Tab · Ctrl+P", "previous group"},
	{"PgUp · PgDn", "scroll the messages"},
	{"Ctrl+C", "quit"},
}

// HelpGuide renders the full help guide as columns of text.
func HelpGuide(width int) string {
	const usageWidth = 34
	usage := lipgloss.NewStyle().Width(usageWidth)
	describe := lipgloss.NewStyle().Width(max(20, width-usageWidth-6))
	row := func(item helpItem, style lipgloss.Style) string {
		return "  " + lipgloss.JoinHorizontal(lipgloss.Top, usage.Render(style.Render(item.usage)), describe.Render(Text.Render(item.what)))
	}

	var out []string
	out = append(out, Brand.Render(">_ Pipechat")+Dim.Render("  ·  end-to-end encrypted terminal chat"), "")
	for _, section := range helpSections {
		out = append(out, Heading.Render(strings.ToUpper(section.title)))
		for _, item := range section.items {
			out = append(out, row(item, Command))
		}
		out = append(out, "")
	}
	out = append(out, Heading.Render("KEYS"))
	for _, item := range helpKeys {
		out = append(out, row(item, Key))
	}
	note := func(text string) string {
		return "  " + strings.ReplaceAll(lipgloss.NewStyle().Width(max(20, width-4)).Render(Text.Render(text)), "\n", "\n  ")
	}
	out = append(out, "",
		Heading.Render("GOOD TO KNOW"),
		note("Groups you make or join on the website appear here by themselves, within a few seconds."),
		note("Anything that is not a /command is sent as a message to the current chat."),
		note("Invite codes and file links contain the decryption key; share them privately."),
		note("Tab finishes a command or a file name in grey; with nothing to finish it switches chats."),
	)
	return strings.Join(out, "\n")
}

// WelcomeLines is shown on the welcome page after logging in.
func WelcomeLines(handle string, fingerprint string) []string {
	return []string{
		"",
		Brand.Render(" ┌─────────┐"),
		Brand.Render(" │ >_      │  ") + Active.Render("Welcome to Pipechat, @"+handle),
		Brand.Render(" └─────────┘  ") + Dim.Render("end-to-end encrypted chat for your terminal"),
		"",
		Heading.Render("Get started"),
		"  " + Command.Render("/create NAME") + Text.Render("   make a group"),
		"  " + Command.Render("/join CODE") + Text.Render("     join one with an invite code"),
		"  " + Command.Render("/dm HANDLE") + Text.Render("     chat privately with someone"),
		"  " + Text.Render("Groups from the website show up here on their own."),
		"",
		"  " + Text.Render("Type ") + Command.Render("/help") + Text.Render(" for every command, or press ") + Key.Render("Tab") + Text.Render(" to switch chats."),
		"",
		Dim.Render("  Your key fingerprint: " + fingerprint),
		"",
	}
}

// Profile is what /whois shows about a person.
type Profile struct {
	Handle string
	Name   string
	Bio    string
	Joined int64 // unix seconds
	Online bool
}

// ProfilePage renders what /whois shows on its page of its own.
func ProfilePage(width int, p Profile, own bool, fingerprint string, keyCheck string) string {
	wrap := lipgloss.NewStyle().Width(max(20, width-4))
	status := Dim.Render("○ offline")
	if p.Online {
		status = Brand.Render("● online")
	}
	label := lipgloss.NewStyle().Width(18).Foreground(lipgloss.Color("244"))

	var out []string
	out = append(out, Brand.Render("@"+p.Handle)+"   "+status)
	if p.Name != "" {
		out = append(out, Active.Render(p.Name))
	}
	out = append(out, "", Heading.Render("BIO"))
	switch {
	case p.Bio != "":
		out = append(out, "  "+strings.ReplaceAll(wrap.Render(Text.Render(p.Bio)), "\n", "\n  "))
	case own:
		out = append(out, "  "+Dim.Render("You have not written one yet. Set it with ")+Command.Render("/bio TEXT")+Dim.Render("."))
	default:
		out = append(out, "  "+Dim.Render("Nothing written yet."))
	}
	out = append(out, "", Heading.Render("DETAILS"))
	if p.Joined > 0 {
		out = append(out, "  "+label.Render("Member since")+Text.Render(time.Unix(p.Joined, 0).Format("2 January 2006")))
	}
	out = append(out, "  "+label.Render("Key fingerprint")+Text.Render(fingerprint))
	if keyCheck != "" {
		out = append(out, "  "+label.Render("Key check")+wrap.Width(max(20, width-22)).Render(Text.Render(keyCheck)))
	}
	if own {
		out = append(out, "", Dim.Render("  Change your bio with /bio TEXT, or on the website under Profile."))
	} else {
		out = append(out, "", Dim.Render("  Start a private chat with /dm "+p.Handle+"."))
	}
	return strings.Join(out, "\n")
}

// UpdatePrompt is the page offering a new version of the program.
func UpdatePrompt(width int, current string, latest string, size int64, checksum string) string {
	label := lipgloss.NewStyle().Width(16).Foreground(lipgloss.Color("244"))
	row := func(key string, value string) string { return "  " + label.Render(key) + Text.Render(value) }
	wrap := lipgloss.NewStyle().Width(max(20, width-4))

	out := []string{
		Brand.Render("A new version of Pipechat is ready"),
		"",
		row("You have", "v"+strings.TrimPrefix(current, "v")),
		row("Newest", Active.Render("v"+strings.TrimPrefix(latest, "v"))),
	}
	if size > 0 {
		out = append(out, row("Download", files.FormatSize(size)))
	}
	if checksum != "" {
		out = append(out, row("SHA-256", Truncate(checksum, max(20, width-20))))
	}
	out = append(out, "",
		"  "+strings.ReplaceAll(wrap.Render(Text.Render("Updating downloads the new program, checks it, replaces this one, and starts it again. Your chats, groups and settings are not touched; you only log in again.")), "\n", "\n  "),
		"",
		"  "+Key.Render("Y")+Dim.Render("  update now"),
		"  "+Key.Render("N")+Dim.Render("  not now; ask me again next time"),
		"  "+Key.Render("S")+Dim.Render("  skip this version"),
		"",
		Dim.Render("  You can check any time with /update."),
	)
	return strings.Join(out, "\n")
}

// SearchGroup is the matches found in one chat.
type SearchGroup struct {
	Chat string
	Hits []search.Hit
}

// SearchResults draws what /search found, grouped by chat, with the matching words marked.
func SearchResults(width int, query string, groups []SearchGroup, total int, shown int) string {
	marked := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("16")).Background(lipgloss.Color("222"))
	wrap := lipgloss.NewStyle().Width(max(20, width-6))

	var out []string
	found := fmt.Sprintf("%d match", total)
	if total != 1 {
		found += "es"
	}
	out = append(out, Brand.Render("Search: ")+Active.Render(query), Dim.Render(found+" in "+fmt.Sprint(len(groups))+" chat(s)"), "")
	if total == 0 {
		return strings.Join(append(out, Dim.Render("  Nothing said in these chats contains that text.")), "\n")
	}
	for _, group := range groups {
		out = append(out, Heading.Render(group.Chat)+Dim.Render(fmt.Sprintf("  (%d)", len(group.Hits))))
		lastDay := ""
		for _, hit := range group.Hits {
			day := hit.When.Format("2 Jan 2006")
			stamp := hit.When.Format("15:04")
			if day != lastDay {
				out = append(out, "  "+Dim.Render(day))
				lastDay = day
			}
			who := Dim.Render("·")
			if hit.Handle != "" {
				who = Other.Render("@" + hit.Handle)
			}
			text := hit.Text[:hit.Start] + marked.Render(hit.Text[hit.Start:hit.End]) + hit.Text[hit.End:]
			line := Dim.Render(stamp) + " " + who + "  " + Text.Render(text)
			out = append(out, "    "+strings.ReplaceAll(wrap.Render(line), "\n", "\n        "))
		}
		out = append(out, "")
	}
	if shown < total {
		out = append(out, Dim.Render(fmt.Sprintf("  Showing the first %d of %d. Search for more of the words to narrow it down.", shown, total)))
	}
	return strings.Join(out, "\n")
}
