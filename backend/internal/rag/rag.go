// Package rag is a deliberately tiny retrieval layer over the partner
// contracts and the ops playbook: markdown split by "## " section, scored with
// BM25. In production this would be embeddings + a vector store (e.g.
// pgvector) with hybrid keyword search, but the contract with the agent is the
// same: return a few short chunks with stable IDs the agent must cite.
package rag

import (
	"embed"
	"math"
	"path"
	"sort"
	"strings"
	"unicode"
)

//go:embed policies/*.md
var policyFS embed.FS

type Chunk struct {
	ID       string `json:"id"` // "<doc>#<section>", the citation key
	DocTitle string `json:"doc_title"`
	Section  string `json:"section"`
	Text     string `json:"text"`
	terms    map[string]int
	length   int
}

type Hit struct {
	ID       string  `json:"id"`
	DocTitle string  `json:"doc_title"`
	Section  string  `json:"section"`
	Text     string  `json:"text"`
	Score    float64 `json:"score"`
}

type Index struct {
	chunks []*Chunk
	df     map[string]int
	avgLen float64
}

func Load() (*Index, error) {
	entries, err := policyFS.ReadDir("policies")
	if err != nil {
		return nil, err
	}
	ix := &Index{df: map[string]int{}}
	total := 0
	for _, e := range entries {
		raw, err := policyFS.ReadFile(path.Join("policies", e.Name()))
		if err != nil {
			return nil, err
		}
		doc := strings.TrimSuffix(e.Name(), ".md")
		lines := strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
		title := doc
		var cur *Chunk
		flush := func() {
			if cur == nil {
				return
			}
			cur.Text = strings.TrimSpace(cur.Text)
			toks := tokenize(cur.Section + " " + cur.Text)
			cur.terms = map[string]int{}
			for _, t := range toks {
				cur.terms[t]++
			}
			cur.length = len(toks)
			total += cur.length
			for t := range cur.terms {
				ix.df[t]++
			}
			ix.chunks = append(ix.chunks, cur)
		}
		for _, l := range lines {
			switch {
			case strings.HasPrefix(l, "# "):
				title = strings.TrimPrefix(l, "# ")
			case strings.HasPrefix(l, "## "):
				flush()
				sec := strings.TrimPrefix(l, "## ")
				cur = &Chunk{ID: doc + "#" + slug(sec), DocTitle: title, Section: sec}
			case cur != nil:
				cur.Text += l + "\n"
			}
		}
		flush()
	}
	if n := len(ix.chunks); n > 0 {
		ix.avgLen = float64(total) / float64(n)
	}
	return ix, nil
}

// Search returns the top k chunks. partner (optional) restricts results to
// that partner's contract plus the shared ops playbook.
func (ix *Index) Search(query, partner string, k int) []Hit {
	const k1, b = 1.2, 0.75
	q := tokenize(query)
	n := float64(len(ix.chunks))
	partner = strings.ToLower(strings.TrimSpace(partner))
	var hits []Hit
	for _, c := range ix.chunks {
		if partner != "" && !strings.HasPrefix(c.ID, partner) && !strings.HasPrefix(c.ID, "ops-playbook") {
			continue
		}
		score := 0.0
		for _, t := range q {
			tf := float64(c.terms[t])
			if tf == 0 {
				continue
			}
			idf := math.Log(1 + (n-float64(ix.df[t])+0.5)/(float64(ix.df[t])+0.5))
			score += idf * tf * (k1 + 1) / (tf + k1*(1-b+b*float64(c.length)/ix.avgLen))
		}
		if score > 0 {
			hits = append(hits, Hit{ID: c.ID, DocTitle: c.DocTitle, Section: c.Section, Text: c.Text, Score: math.Round(score*100) / 100})
		}
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
	if len(hits) > k {
		hits = hits[:k]
	}
	return hits
}

var stop = map[string]bool{"the": true, "and": true, "for": true, "are": true, "with": true, "that": true, "this": true, "not": true, "when": true, "can": true, "what": true, "how": true, "does": true, "from": true, "per": true, "any": true, "its": true, "must": true}

func tokenize(s string) []string {
	f := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	out := f[:0]
	for _, t := range f {
		if len(t) < 3 || stop[t] {
			continue
		}
		t = strings.TrimSuffix(t, "s") // naive stemming: "days" == "day"
		out = append(out, t)
	}
	return out
}

func slug(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.TrimSuffix(b.String(), "-")
}
