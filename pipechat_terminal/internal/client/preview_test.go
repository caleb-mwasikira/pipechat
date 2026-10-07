package client

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"pipechat/internal/crypto"
)

// typeInto types text into the chat input, key by key, as a person would.
func typeInto(m *Model, text string) {
	m.input.Reset()
	for _, character := range text {
		m.updateChat(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{character}})
	}
}

func makeTree(t *testing.T) string {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "docs", "deep"), 0o755)
	os.MkdirAll(filepath.Join(root, "My Music"), 0o755)
	os.WriteFile(filepath.Join(root, "docs", "notes.txt"), []byte("first line\nsecond line\n"), 0o644)
	os.WriteFile(filepath.Join(root, "docs", "deep", "a.bin"), []byte{0, 1, 2, 3}, 0o644)
	os.WriteFile(filepath.Join(root, "report.pdf"), []byte("%PDF-1.4 ..."), 0o644)
	os.WriteFile(filepath.Join(root, "readme.md"), []byte("# hi\n"), 0o644)
	os.WriteFile(filepath.Join(root, ".hidden"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(root, "My Music", "song one.mp3"), []byte("ID3"), 0o644)
	return root
}

func TestPathsComplete(t *testing.T) {
	root := makeTree(t)
	m := loggedInModel(t)

	typeInto(m, "/upload "+root+"/re")
	if got := m.input.CurrentSuggestion(); got != "/upload "+root+"/readme.md" {
		t.Fatalf("first match: %q (all: %v)", got, m.input.MatchedSuggestions())
	}
	m.updateChat(tea.KeyMsg{Type: tea.KeyDown})
	if got := m.input.CurrentSuggestion(); got != "/upload "+root+"/report.pdf" {
		t.Fatalf("second match: %q", got)
	}

	// A folder completes with a slash, and then offers what is inside it.
	typeInto(m, "/upload "+root+"/do")
	m.updateChat(tea.KeyMsg{Type: tea.KeyTab})
	if m.input.Value() != "/upload "+root+"/docs/" {
		t.Fatalf("Tab should finish the folder, got %q", m.input.Value())
	}
	if got := m.input.CurrentSuggestion(); got != "/upload "+root+"/docs/deep/" {
		t.Fatalf("inside the folder: %q (all: %v)", got, m.input.MatchedSuggestions())
	}

	// Hidden files appear only when asked for with a dot.
	typeInto(m, "/upload "+root+"/")
	for _, suggestion := range m.input.MatchedSuggestions() {
		if strings.Contains(suggestion, ".hidden") {
			t.Fatal("hidden files should not be offered")
		}
	}
	typeInto(m, "/upload "+root+"/.h")
	if m.input.CurrentSuggestion() != "/upload "+root+"/.hidden" {
		t.Fatalf("hidden file asked for: %q", m.input.CurrentSuggestion())
	}

	// A name with a space is offered in quotes, and a folder's quote stays open to go on inside it.
	typeInto(m, `/upload "`+root+"/My")
	if got := m.input.CurrentSuggestion(); got != `/upload "`+root+`/My Music/` {
		t.Fatalf("quoted folder: %q", got)
	}
	m.updateChat(tea.KeyMsg{Type: tea.KeyTab})
	if got := m.input.CurrentSuggestion(); got != `/upload "`+root+`/My Music/song one.mp3"` {
		t.Fatalf("file in the quoted folder: %q", got)
	}
}

func TestPathCompletionOnlyWhereAPathGoes(t *testing.T) {
	root := makeTree(t)
	m := loggedInModel(t)

	typeInto(m, "/upload -name "+root+"/re") // after -name comes a short name, not a path
	if m.suggestionShown() {
		t.Fatalf("a short name is not a path: %v", m.input.MatchedSuggestions())
	}
	typeInto(m, "/upload "+root+"/readme.md "+root+"/re") // one path is enough
	if m.suggestionShown() {
		t.Fatal("a second path should not be completed")
	}
	typeInto(m, "/upload -name pic "+root+"/re") // the path may follow the name
	if !m.suggestionShown() {
		t.Fatal("the path after -name x should complete")
	}

	// /download completes only folders, and only for its second argument.
	typeInto(m, "/download notes "+root+"/")
	for _, suggestion := range m.input.MatchedSuggestions() {
		if strings.HasSuffix(suggestion, ".pdf") || strings.HasSuffix(suggestion, ".md") {
			t.Fatalf("files offered where only a folder fits: %s", suggestion)
		}
	}
	if m.input.CurrentSuggestion() != "/download notes "+root+"/My Music/" && !strings.HasSuffix(m.input.CurrentSuggestion(), "/") {
		t.Fatalf("expected a folder, got %q", m.input.CurrentSuggestion())
	}
	typeInto(m, "/download "+root+"/") // the first argument is a name or link
	if m.suggestionShown() {
		t.Fatal("the first /download argument is not a path")
	}

	// Typing something else clears the file suggestions.
	typeInto(m, "/pl")
	if m.input.CurrentSuggestion() != "/play" {
		t.Fatalf("commands should still complete: %q", m.input.CurrentSuggestion())
	}
}

func TestUploadPreviewDescribesWhatIsShared(t *testing.T) {
	root := makeTree(t)
	var picture bytes.Buffer
	canvas := image.NewRGBA(image.Rect(0, 0, 40, 30))
	canvas.Set(1, 1, color.White)
	png.Encode(&picture, canvas)
	os.WriteFile(filepath.Join(root, "photo.png"), picture.Bytes(), 0o644)

	m := loggedInModel(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 60})
	secret, _ := crypto.NewSecret("crew")
	m.groups = append(m.groups, &groupState{secret: secret})
	m.switchTo(1)

	look := func(path string) string {
		t.Helper()
		m.input.SetValue("/upload " + path)
		m.updateChat(tea.KeyMsg{Type: tea.KeyEnter})
		if !m.showHelp || m.pendingSend == nil {
			t.Fatalf("/upload %s should open a preview", path)
		}
		view := m.View()
		m.updateChat(tea.KeyMsg{Type: tea.KeyEsc})
		return view
	}

	text := look(filepath.Join(root, "docs", "notes.txt"))
	for _, want := range []string{"Ready to upload?", "notes.txt", "crew", "text (txt)", "first line", "second line", "Enter or y to upload", "Esc or n to cancel"} {
		if !strings.Contains(text, want) {
			t.Errorf("text preview lacks %q:\n%s", want, text)
		}
	}
	if picture := look(filepath.Join(root, "photo.png")); !strings.Contains(picture, "PNG image, 40 × 30 pixels") {
		t.Errorf("image preview:\n%s", picture)
	}
	if binary := look(filepath.Join(root, "docs", "deep", "a.bin")); !strings.Contains(binary, "binary file") || strings.Contains(binary, "PREVIEW") {
		t.Errorf("binary preview:\n%s", binary)
	}
	if pdf := look(filepath.Join(root, "report.pdf")); !strings.Contains(pdf, "PDF document") {
		t.Errorf("pdf preview:\n%s", pdf)
	}
	folder := look(filepath.Join(root, "docs"))
	for _, want := range []string{"folder", "2 files", "1 folders", "notes.txt", "deep/"} {
		if !strings.Contains(folder, want) {
			t.Errorf("folder preview lacks %q:\n%s", want, folder)
		}
	}

	// Cancelling sends nothing, and says so.
	m.input.SetValue("/upload " + filepath.Join(root, "readme.md"))
	m.updateChat(tea.KeyMsg{Type: tea.KeyEnter})
	m.updateChat(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if m.showHelp || m.busy || !strings.Contains(entriesText(m.groups[1]), "upload cancelled") {
		t.Fatalf("cancel: page %v, busy %v:\n%s", m.showHelp, m.busy, entriesText(m.groups[1]))
	}

	// A path that does not exist is refused at once, without a page.
	m.input.SetValue("/upload " + filepath.Join(root, "missing"))
	m.updateChat(tea.KeyMsg{Type: tea.KeyEnter})
	if m.showHelp || !strings.Contains(entriesText(m.groups[1]), "cannot read") {
		t.Fatal("a missing file should not open a preview")
	}
}
