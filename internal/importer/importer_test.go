package importer

import (
	"bytes"
	"strings"
	"testing"
)

func TestImportConstitutionTextSplitsPreambleAndArticle(t *testing.T) {
	output, err := ImportFile(Options{
		InputPath:           "testdata/constitution-small.txt",
		DocumentID:          "jp-constitution-test",
		CampaignID:          "constitution-test",
		DocumentFixturePath: "content/documents/constitution-test.json",
	})
	if err != nil {
		t.Fatalf("import constitution fixture: %v", err)
	}

	if got, want := output.Document.Title.JA, "日本国憲法"; got != want {
		t.Fatalf("document title = %q, want %q", got, want)
	}
	if got, want := len(output.Document.Sections), 2; got != want {
		t.Fatalf("section count = %d, want %d", got, want)
	}

	preamble := output.Document.Sections[0]
	if got, want := preamble.SectionID, "preamble-1"; got != want {
		t.Fatalf("preamble section id = %q, want %q", got, want)
	}
	if got, want := preamble.Kind, "preamble_paragraph"; got != want {
		t.Fatalf("preamble kind = %q, want %q", got, want)
	}
	if !strings.Contains(preamble.OfficialText, "よつて") || !strings.Contains(preamble.OfficialText, "ないやうに") {
		t.Fatalf("preamble official text did not preserve historical orthography: %q", preamble.OfficialText)
	}
	if strings.Contains(preamble.OfficialText, "よって") || strings.Contains(preamble.OfficialText, "ないように") {
		t.Fatalf("preamble official text was modernized: %q", preamble.OfficialText)
	}

	article := output.Document.Sections[1]
	if got, want := article.SectionID, "article-1"; got != want {
		t.Fatalf("article section id = %q, want %q", got, want)
	}
	if article.Article == nil || article.Article.Number != 1 {
		t.Fatalf("article metadata = %#v, want article 1", article.Article)
	}
	if article.Chapter == nil || article.Chapter.Number != 1 || article.Chapter.TitleJA != "天皇" {
		t.Fatalf("chapter metadata = %#v, want chapter 1 天皇", article.Chapter)
	}
	wantArticle := "第一条　天皇は、日本国の象徴であり日本国民統合の象徴であつて、この地位は、主権の存する日本国民の総意に基く。"
	if article.OfficialText != wantArticle {
		t.Fatalf("article official text = %q, want %q", article.OfficialText, wantArticle)
	}
	if article.OfficialBody != "天皇は、日本国の象徴であり日本国民統合の象徴であつて、この地位は、主権の存する日本国民の総意に基く。" {
		t.Fatalf("article official body was not split from heading: %q", article.OfficialBody)
	}

	if got, want := len(output.Campaign.Levels), 2; got != want {
		t.Fatalf("campaign level count = %d, want %d", got, want)
	}
	level := output.Campaign.Levels[0]
	if got, want := level.DocumentSectionID, "preamble-1"; got != want {
		t.Fatalf("level section id = %q, want %q", got, want)
	}
	if !level.NeedsReview {
		t.Fatalf("imported level should keep needs_review=true until learner hints are reviewed")
	}
	if len(level.ReadingReviewFlags) != 1 || level.ReadingReviewFlags[0] != defaultReviewFlag {
		t.Fatalf("level reading review flags = %#v, want %q", level.ReadingReviewFlags, defaultReviewFlag)
	}
	if len(level.PrerequisiteCards) != 0 || len(level.LearnerHints) != 0 {
		t.Fatalf("importer should keep learner-layer data separate and empty by default: cards=%#v hints=%#v", level.PrerequisiteCards, level.LearnerHints)
	}
	if len(output.Campaign.ReadingReviewFlags) == 0 || output.Campaign.ReadingReviewFlags[0].Flag != defaultReviewFlag {
		t.Fatalf("campaign review flags = %#v, want first flag %q", output.Campaign.ReadingReviewFlags, defaultReviewFlag)
	}
}

func TestImportMarkdownSections(t *testing.T) {
	source := "# Journal Lesson\n\n## Key Readings\n日 changes reading by word.\n\n## Time Words\n今日 and 明日.\n"
	output, err := ImportText(source, Options{
		InputPath:           "journal.md",
		DocumentFixturePath: "content/documents/journal.json",
	})
	if err != nil {
		t.Fatalf("import markdown: %v", err)
	}

	if got, want := output.Document.DocumentID, "journal"; got != want {
		t.Fatalf("document id = %q, want %q", got, want)
	}
	if got, want := output.Document.Title.JA, "Journal Lesson"; got != want {
		t.Fatalf("title = %q, want %q", got, want)
	}
	if got, want := output.Document.Sections[0].SectionID, "key-readings"; got != want {
		t.Fatalf("first section id = %q, want %q", got, want)
	}
	if got, want := output.Document.Sections[0].OfficialText, "日 changes reading by word."; got != want {
		t.Fatalf("first section official text = %q, want %q", got, want)
	}
}

func TestImportRejectsUnsupportedExtension(t *testing.T) {
	_, err := ImportFile(Options{InputPath: "content.json"})
	if err == nil {
		t.Fatalf("ImportFile accepted unsupported extension")
	}
	if !strings.Contains(err.Error(), "expected .md or .txt") {
		t.Fatalf("unsupported extension error = %v", err)
	}
}

func TestWriteJSONKeepsEmptyLearnerArrays(t *testing.T) {
	output, err := ImportFile(Options{
		InputPath:           "testdata/constitution-small.txt",
		DocumentID:          "jp-constitution-test",
		CampaignID:          "constitution-test",
		DocumentFixturePath: "content/documents/constitution-test.json",
	})
	if err != nil {
		t.Fatalf("import constitution fixture: %v", err)
	}

	var buf bytes.Buffer
	if err := WriteJSON(&buf, output.Campaign); err != nil {
		t.Fatalf("write campaign json: %v", err)
	}
	got := buf.String()
	for _, want := range []string{`"prerequisite_cards": []`, `"learner_hints": []`, `"needs_review": true`, `"reading_review_flags": [`} {
		if !strings.Contains(got, want) {
			t.Fatalf("campaign JSON missing %s:\n%s", want, got)
		}
	}
}
