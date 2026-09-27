package render

import (
	"strings"
	"testing"

	"github.com/konorlevich/fitto-club/internal/content"
	"github.com/konorlevich/fitto-club/internal/store"
)

// Folding a long review must never lose or alter the author's words: the
// shown part plus the folded part is exactly the original text.
func TestReviewSplitKeepsEveryWord(t *testing.T) {
	p := &Page{}
	for _, r := range store.SeedSnapshot().Reviews {
		parts := p.RevShort(r).Split()
		joined := strings.Join(append(append([]string{}, parts.Head...), parts.Tail...), " ")
		want := strings.Join(p.Paras(r.Text), " ")
		if strings.Join(strings.Fields(joined), " ") != strings.Join(strings.Fields(want), " ") {
			t.Errorf("%s: split changed the text", r.ID)
		}
		if len(parts.Head) == 0 {
			t.Errorf("%s: nothing left visible", r.ID)
		}
	}
}

func TestReviewSplitThresholds(t *testing.T) {
	p := &Page{}
	short := content.Review{Text: strings.Repeat("Short sentence here. ", 10)} // ~210 runes
	if parts := p.RevShort(short).Split(); parts.Tail != nil {
		t.Error("a short review must not be folded")
	}
	long := content.Review{Text: strings.Repeat("A sentence of about forty characters. ", 20)}
	parts := p.RevShort(long).Split()
	if parts.Tail == nil || !strings.HasSuffix(parts.Head[0], ".") {
		t.Errorf("a long single paragraph must be cut at a sentence end: %q", parts.Head)
	}
	if parts := p.Rev(long).Split(); parts.Tail != nil {
		t.Error("coach pages show reviews whole")
	}
}
