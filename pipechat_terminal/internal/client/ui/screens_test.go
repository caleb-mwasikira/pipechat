package ui

import (
	"strings"
	"testing"
)

func TestHelpGuideListsEveryCommandWithinTheWidth(t *testing.T) {
	guide := HelpGuide(80)
	for _, want := range []string{"GROUPS", "MUSIC", "FILES", "KEYS", "/create NAME", "/play NUMBER", "/upload PATH", "/kick HANDLE", "/close"} {
		if !strings.Contains(guide, want) {
			t.Errorf("the guide lacks %q", want)
		}
	}
	for _, line := range strings.Split(stripStyles(guide), "\n") {
		if len([]rune(line)) > 80 {
			t.Errorf("a line is wider than the screen: %q", line)
		}
	}
}

func TestProfilePage(t *testing.T) {
	page := stripStyles(ProfilePage(80, Profile{Handle: "alice", Name: "Alice A", Bio: "Writes Go.", Joined: 1790000000, Online: true}, false, "aaaa bbbb", "matches the key you saved"))
	for _, want := range []string{"@alice", "● online", "Alice A", "Writes Go.", "Member since", "aaaa bbbb", "matches the key you saved", "/dm alice"} {
		if !strings.Contains(page, want) {
			t.Errorf("profile lacks %q:\n%s", want, page)
		}
	}
	own := stripStyles(ProfilePage(80, Profile{Handle: "me"}, true, "cccc", ""))
	if !strings.Contains(own, "○ offline") || !strings.Contains(own, "/bio TEXT") || strings.Contains(own, "Key check") {
		t.Errorf("own profile:\n%s", own)
	}
}

func TestWelcome(t *testing.T) {
	text := stripStyles(strings.Join(WelcomeLines("ann", "1111 2222"), "\n"))
	for _, want := range []string{"Welcome to Pipechat, @ann", "/create NAME", "/help", "1111 2222"} {
		if !strings.Contains(text, want) {
			t.Errorf("welcome lacks %q", want)
		}
	}
}

func TestOnlinePanel(t *testing.T) {
	panel := stripStyles(OnlinePanel([]string{"bob", "carol"}, "ann", true, 10))
	for _, want := range []string{"Online (3)", "@ann (you)", "@bob", "@carol"} {
		if !strings.Contains(panel, want) {
			t.Errorf("panel lacks %q:\n%s", want, panel)
		}
	}
	if strings.Contains(stripStyles(OnlinePanel(nil, "ann", false, 10)), "@ann") {
		t.Error("you are not listed while disconnected")
	}
	if OnlinePanel([]string{"bob"}, "ann", true, 2) != "" {
		t.Error("no room, no panel")
	}
	crowded := stripStyles(OnlinePanel([]string{"a1", "a2", "a3", "a4", "a5", "a6"}, "ann", true, 5))
	if !strings.Contains(crowded, "more") || strings.Count(crowded, "\n") > 5 {
		t.Errorf("a crowd should be cut short:\n%s", crowded)
	}
}
