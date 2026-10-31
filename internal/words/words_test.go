package words

import "testing"

func TestLoad(t *testing.T) {
	b, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range Lengths {
		for d := Easy; d <= Hard; d++ {
			if len(b.pools[n][d]) < 100 {
				t.Errorf("length %d difficulty %d: only %d words", n, d, len(b.pools[n][d]))
			}
		}
	}
}

func TestBank(t *testing.T) {
	b, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !b.IsWord("house") {
		t.Error("house should be a word")
	}
	if b.IsWord("zzzzz") {
		t.Error("zzzzz should not be a word")
	}
	if !b.SecretAllowed("house", Easy) {
		t.Error("house should be an easy secret")
	}
	if b.SecretAllowed("houses", Hard) {
		t.Error("plurals should not be secrets")
	}
	for d := Easy; d <= Hard; d++ {
		w, err := b.RandomSecret(5, d, true)
		if err != nil {
			t.Fatal(err)
		}
		if !HasUniqueLetters(w) || !b.SecretAllowed(w, d) {
			t.Errorf("RandomSecret(5, %d, true) = %q", d, w)
		}
	}
}

func TestHasUniqueLetters(t *testing.T) {
	cases := map[string]bool{"crane": true, "apple": false, "abcdef": true, "aa": false}
	for w, want := range cases {
		if got := HasUniqueLetters(w); got != want {
			t.Errorf("HasUniqueLetters(%q) = %v, want %v", w, got, want)
		}
	}
}
