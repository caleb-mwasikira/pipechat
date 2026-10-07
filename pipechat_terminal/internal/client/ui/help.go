package ui

import (
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
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
		{"/next · /stop", "skip to the next song · stop the music"},
		{"/shuffle [on|off]", "play in random order"},
		{"/repeat off|one|all", "repeat nothing, this song, or the whole library"},
	}},
	{"Screen", []helpItem{
		{"/clear", "clear this chat's messages from view"},
		{"/help", "show this guide"},
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
