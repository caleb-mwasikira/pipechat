package client

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	brandStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("49"))
	headingStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("49"))
	commandStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("87"))
	keyStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("222"))
	textStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
)

// One help entry: what to type, and what it does.
type helpItem struct{ usage, what string }

type helpSection struct {
	title string
	items []helpItem
}

var helpSections = []helpSection{
	{"Groups", []helpItem{
		{"/create NAME", "make a new group (1-24 characters)"},
		{"/join CODE", "join a group with an invite code"},
		{"/code", "show this group's invite code; share it privately"},
		{"/invite EMAIL", "email the invite code to someone"},
		{"/groups", "list your groups"},
		{"/switch N", "go to group number N from /groups"},
		{"/kick HANDLE", "remove a member (only the group's creator can)"},
		{"/leave", "leave the current group"},
	}},
	{"Direct messages", []helpItem{
		{"/dm HANDLE", "start a private chat with someone"},
		{"/whois HANDLE", "show their key fingerprint (/me shows yours)"},
	}},
	{"Files", []helpItem{
		{"/upload PATH [-name SHORT]", "send a file or folder, encrypted, to this chat"},
		{"/download SHORT|LINK [DIR]", "save a shared file"},
		{"/files", "list files shared with you"},
		{"/link SHORT", "show a file's download link (it holds the key)"},
		{"/share SHORT", "post a file you already have to this chat"},
	}},
	{"Screen", []helpItem{
		{"/clear", "clear this chat's messages from view"},
		{"/help", "show this guide"},
		{"/exit", "quit (/quit works too)"},
	}},
}

var helpKeys = []helpItem{
	{"Tab · Ctrl+N", "next group"},
	{"Shift+Tab · Ctrl+P", "previous group"},
	{"PgUp · PgDn", "scroll the messages"},
	{"Ctrl+C", "quit"},
}

// helpGuide renders the full help guide as columns of text.
func helpGuide(width int) string {
	const usageWidth = 30
	usage := lipgloss.NewStyle().Width(usageWidth)
	describe := lipgloss.NewStyle().Width(max(20, width-usageWidth-6))
	row := func(item helpItem, style lipgloss.Style) string {
		return "  " + lipgloss.JoinHorizontal(lipgloss.Top, usage.Render(style.Render(item.usage)), describe.Render(textStyle.Render(item.what)))
	}

	var out []string
	out = append(out, brandStyle.Render(">_ Pipechat")+dimStyle.Render("  ·  end-to-end encrypted terminal chat"), "")
	for _, section := range helpSections {
		out = append(out, headingStyle.Render(strings.ToUpper(section.title)))
		for _, item := range section.items {
			out = append(out, row(item, commandStyle))
		}
		out = append(out, "")
	}
	out = append(out, headingStyle.Render("KEYS"))
	for _, item := range helpKeys {
		out = append(out, row(item, keyStyle))
	}
	out = append(out, "",
		headingStyle.Render("GOOD TO KNOW"),
		"  "+textStyle.Render("Groups you make or join on the website appear here by themselves, within a few seconds."),
		"  "+textStyle.Render("Anything that is not a /command is sent as a message to the current chat."),
		"  "+textStyle.Render("Invite codes and file links contain the decryption key; share them privately."),
	)
	return strings.Join(out, "\n")
}

// welcomeLines is shown on the welcome page after logging in.
func welcomeLines(handle string, fingerprint string) []string {
	return []string{
		"",
		brandStyle.Render(" ┌─────────┐"),
		brandStyle.Render(" │ >_      │  ") + activeStyle.Render("Welcome to Pipechat, @"+handle),
		brandStyle.Render(" └─────────┘  ") + dimStyle.Render("end-to-end encrypted chat for your terminal"),
		"",
		headingStyle.Render("Get started"),
		"  " + commandStyle.Render("/create NAME") + textStyle.Render("   make a group"),
		"  " + commandStyle.Render("/join CODE") + textStyle.Render("     join one with an invite code"),
		"  " + commandStyle.Render("/dm HANDLE") + textStyle.Render("     chat privately with someone"),
		"  " + textStyle.Render("Groups from the website show up here on their own."),
		"",
		"  " + textStyle.Render("Type ") + commandStyle.Render("/help") + textStyle.Render(" for every command, or press ") + keyStyle.Render("Tab") + textStyle.Render(" to switch chats."),
		"",
		dimStyle.Render("  Your key fingerprint: " + fingerprint),
		"",
	}
}
