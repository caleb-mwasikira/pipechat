package client

import (
	"os"
	"path/filepath"
	"testing"
)

func seqs(records []historyRecord) []uint64 {
	var out []uint64
	for _, record := range records {
		out = append(out, record.Seq)
	}
	return out
}

func sameSeqs(t *testing.T, got []historyRecord, want ...uint64) {
	t.Helper()
	have := seqs(got)
	if len(have) != len(want) {
		t.Fatalf("got %v, want %v", have, want)
	}
	for i := range want {
		if have[i] != want[i] {
			t.Fatalf("got %v, want %v", have, want)
		}
	}
}

func TestHistoryKeepsMessagesBetweenRuns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	h, _ := LoadHistory(path)
	h.Add("ann/g1", 30, 3, []byte("c"))
	h.Add("ann/g1", 10, 1, []byte("a")) // out of order
	h.Add("ann/g1", 20, 2, []byte("b"))
	h.Add("ann/g1", 20, 2, []byte("b")) // twice
	h.Add("bob/g1", 99, 9, []byte("z"))

	again, err := LoadHistory(path)
	if err != nil {
		t.Fatal(err)
	}
	sameSeqs(t, again.Messages("ann/g1"), 10, 20, 30)
	sameSeqs(t, again.Messages("bob/g1"), 99)
	if again.Last("ann/g1") != 30 || again.Last("nobody/g1") != 0 {
		t.Fatalf("last: %d", again.Last("ann/g1"))
	}
	if info, _ := os.Stat(path); info.Mode().Perm() != 0o600 {
		t.Fatalf("the file should be private: %v", info.Mode())
	}
}

func TestHistoryClearHidesButRemembersHowFarItGot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	h, _ := LoadHistory(path)
	h.Add("ann/g1", 10, 1, []byte("a"))
	h.Add("ann/g1", 20, 2, []byte("b"))
	h.Clear("ann/g1")
	h.Add("ann/g1", 30, 3, []byte("c"))

	again, _ := LoadHistory(path)
	sameSeqs(t, again.Messages("ann/g1"), 30)
	if again.Last("ann/g1") != 30 {
		t.Fatal("must still know how far it got")
	}

	again.Clear("ann/g1")
	third, _ := LoadHistory(path)
	if len(third.Messages("ann/g1")) != 0 || third.Last("ann/g1") != 30 {
		t.Fatalf("cleared chat: %v last %d", seqs(third.Messages("ann/g1")), third.Last("ann/g1"))
	}
}

func TestHistoryForgetRemovesOnlyThatChat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	h, _ := LoadHistory(path)
	h.Add("ann/g1", 10, 1, []byte("a"))
	h.Add("ann/g2", 11, 1, []byte("b"))
	h.Forget("ann/g1")

	again, _ := LoadHistory(path)
	if len(again.Messages("ann/g1")) != 0 || again.Last("ann/g1") != 0 {
		t.Fatal("g1 should be gone")
	}
	sameSeqs(t, again.Messages("ann/g2"), 11)
}

func TestHistoryKeepsOnlyTheNewestAndStaysSmall(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	h, _ := LoadHistory(path)
	for seq := uint64(1); seq <= historyKept+3500; seq++ {
		h.Add("ann/g1", seq, 0, []byte("x"))
	}
	again, _ := LoadHistory(path)
	got := again.Messages("ann/g1")
	if len(got) != historyKept || got[0].Seq != 3501 {
		t.Fatalf("kept %d, first %d", len(got), got[0].Seq)
	}
	third, _ := LoadHistory(path) // the second load tidied the file
	if third.lines != historyKept {
		t.Fatalf("file has %d lines, want %d", third.lines, historyKept)
	}
}

func TestHistorySkipsADamagedLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	os.WriteFile(path, []byte(`{"k":"ann/g1","s":5,"c":"YQ=="}`+"\n"+`{"k":"ann/g1","s":6,"c":"Yg`), 0o600)
	h, err := LoadHistory(path)
	if err != nil {
		t.Fatal(err)
	}
	sameSeqs(t, h.Messages("ann/g1"), 5)
}
