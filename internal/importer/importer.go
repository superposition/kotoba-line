package importer

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

const (
	defaultSchemaVersion = 1
	defaultReviewFlag    = "needs_native_review"
)

var (
	articleHeadingRE = regexp.MustCompile(`^第([0-9一二三四五六七八九十百千〇零]+)条(?:[[:space:]　]*(.*))?$`)
	chapterHeadingRE = regexp.MustCompile(`^第([0-9一二三四五六七八九十百千〇零]+)章(?:[[:space:]　]*(.*))?$`)
)

type Options struct {
	InputPath           string
	DocumentID          string
	CampaignID          string
	TitleJA             string
	TitleEN             string
	CampaignTitleJA     string
	CampaignTitleEN     string
	DocumentFixturePath string
	SourceDate          string
	StationPrefix       string
}

type Output struct {
	Document DocumentFixture
	Campaign CampaignFixture
}

type LocalizedTitle struct {
	JA string `json:"ja,omitempty"`
	EN string `json:"en,omitempty"`
}

type DocumentFixture struct {
	SchemaVersion int               `json:"schema_version"`
	DocumentID    string            `json:"document_id"`
	Title         LocalizedTitle    `json:"title"`
	SourcePath    string            `json:"source_path,omitempty"`
	SourceDate    string            `json:"source_date,omitempty"`
	Preservation  PreservationNote  `json:"preservation"`
	Sections      []DocumentSection `json:"sections"`
}

type PreservationNote struct {
	OfficialTextLocked bool   `json:"official_text_locked"`
	Normalization      string `json:"normalization"`
	Note               string `json:"note"`
}

type DocumentSection struct {
	SectionID    string         `json:"section_id"`
	Kind         string         `json:"kind"`
	Chapter      *ChapterInfo   `json:"chapter,omitempty"`
	Article      *ArticleInfo   `json:"article,omitempty"`
	Label        LocalizedTitle `json:"label"`
	OfficialText string         `json:"official_text"`
	OfficialBody string         `json:"official_body,omitempty"`
	SourceRefs   []string       `json:"source_refs,omitempty"`
}

type ChapterInfo struct {
	Number  int    `json:"number"`
	TitleJA string `json:"title_ja,omitempty"`
	TitleEN string `json:"title_en,omitempty"`
}

type ArticleInfo struct {
	Number  int    `json:"number"`
	TitleJA string `json:"title_ja,omitempty"`
}

type CampaignFixture struct {
	SchemaVersion      int                 `json:"schema_version"`
	CampaignID         string              `json:"campaign_id"`
	DocumentID         string              `json:"document_id"`
	Title              LocalizedTitle      `json:"title"`
	DocumentFixture    string              `json:"document_fixture,omitempty"`
	Status             string              `json:"status"`
	ReadingReviewFlags []ReadingReviewFlag `json:"reading_review_flags"`
	Levels             []CampaignLevel     `json:"levels"`
}

type ReadingReviewFlag struct {
	Flag     string   `json:"flag"`
	Scope    string   `json:"scope,omitempty"`
	Surfaces []string `json:"surfaces,omitempty"`
	Reason   string   `json:"reason"`
}

type CampaignLevel struct {
	LevelID            string             `json:"level_id"`
	DocumentSectionID  string             `json:"document_section_id"`
	Title              LocalizedTitle     `json:"title"`
	Station            string             `json:"station"`
	OfficialTextRef    string             `json:"official_text_ref"`
	PrerequisiteCards  []PrerequisiteCard `json:"prerequisite_cards"`
	LearnerHints       []LearnerHint      `json:"learner_hints"`
	NeedsReview        bool               `json:"needs_review"`
	ReadingReviewFlags []string           `json:"reading_review_flags"`
}

type PrerequisiteCard struct {
	CardID      string   `json:"card_id"`
	Surface     string   `json:"surface"`
	LearnerKana string   `json:"learner_kana,omitempty"`
	Readings    []string `json:"readings,omitempty"`
	Review      string   `json:"review"`
	NeedsReview bool     `json:"needs_review"`
	MeaningHint string   `json:"meaning_hint,omitempty"`
	ReviewFlags []string `json:"review_flags,omitempty"`
}

type LearnerHint struct {
	Surface     string   `json:"surface"`
	LearnerKana string   `json:"learner_kana,omitempty"`
	Readings    []string `json:"readings,omitempty"`
	RomajiHint  string   `json:"romaji_hint,omitempty"`
	MeaningHint string   `json:"meaning_hint,omitempty"`
	ReviewFlags []string `json:"review_flags,omitempty"`
	NeedsReview bool     `json:"needs_review"`
}

type parsedSection struct {
	heading         string
	kind            string
	text            string
	body            string
	articleNumber   int
	chapterNumber   int
	chapterTitleJA  string
	includeHeading  bool
	markdownSection bool
}

type currentSection struct {
	heading         string
	kind            string
	articleNumber   int
	chapterNumber   int
	chapterTitleJA  string
	includeHeading  bool
	markdownSection bool
	lines           []rawLine
}

type rawLine struct {
	text    string
	newline string
}

type heading struct {
	ok             bool
	docTitle       bool
	chapter        bool
	preamble       bool
	article        bool
	genericSection bool
	title          string
	articleNumber  int
	chapterNumber  int
	body           string
	includeLine    bool
	markdown       bool
}

func ImportFile(opts Options) (*Output, error) {
	if strings.TrimSpace(opts.InputPath) == "" {
		return nil, errors.New("input path is required")
	}
	if err := validateInputPath(opts.InputPath); err != nil {
		return nil, err
	}
	info, err := os.Stat(opts.InputPath)
	if err != nil {
		return nil, fmt.Errorf("stat input: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("input path %q is a directory", opts.InputPath)
	}
	data, err := os.ReadFile(opts.InputPath)
	if err != nil {
		return nil, fmt.Errorf("read input: %w", err)
	}
	return ImportText(string(data), opts)
}

func ImportText(source string, opts Options) (*Output, error) {
	if strings.TrimSpace(source) == "" {
		return nil, errors.New("input document is empty")
	}

	sections, title := splitSections(source)
	if len(sections) == 0 {
		return nil, errors.New("input document produced no importable sections")
	}

	documentID := firstNonBlank(opts.DocumentID, slugFromPath(opts.InputPath), "imported-document")
	campaignID := firstNonBlank(opts.CampaignID, documentID)
	titleJA := firstNonBlank(opts.TitleJA, title, documentID)
	titleEN := opts.TitleEN
	campaignTitleJA := firstNonBlank(opts.CampaignTitleJA, titleJA)
	campaignTitleEN := firstNonBlank(opts.CampaignTitleEN, titleEN)
	fixturePath := firstNonBlank(opts.DocumentFixturePath, opts.InputPath)
	stationPrefix := firstNonBlank(opts.StationPrefix, campaignID)

	documentSections := make([]DocumentSection, 0, len(sections))
	usedSectionIDs := map[string]int{}
	for _, section := range sections {
		documentSections = append(documentSections, toDocumentSection(section, usedSectionIDs))
	}

	document := DocumentFixture{
		SchemaVersion: defaultSchemaVersion,
		DocumentID:    documentID,
		Title: LocalizedTitle{
			JA: titleJA,
			EN: titleEN,
		},
		SourcePath: firstNonBlank(opts.InputPath),
		SourceDate: opts.SourceDate,
		Preservation: PreservationNote{
			OfficialTextLocked: true,
			Normalization:      "none",
			Note:               "Preserve source orthography and punctuation exactly as imported. Learner readings, romaji, glosses, chunking, and review status are layered in campaign fixtures instead of editing this text.",
		},
		Sections: documentSections,
	}

	levels := make([]CampaignLevel, 0, len(documentSections))
	for _, section := range documentSections {
		levelID := campaignID + "-" + section.SectionID
		levels = append(levels, CampaignLevel{
			LevelID:           levelID,
			DocumentSectionID: section.SectionID,
			Title:             section.Label,
			Station:           stationPrefix + "-" + section.SectionID,
			OfficialTextRef:   fixturePath + "#sections/" + section.SectionID + "/official_text",
			PrerequisiteCards: []PrerequisiteCard{},
			LearnerHints:      []LearnerHint{},
			NeedsReview:       true,
			ReadingReviewFlags: []string{
				defaultReviewFlag,
			},
		})
	}

	campaign := CampaignFixture{
		SchemaVersion:   defaultSchemaVersion,
		CampaignID:      campaignID,
		DocumentID:      documentID,
		Title:           LocalizedTitle{JA: campaignTitleJA, EN: campaignTitleEN},
		DocumentFixture: fixturePath,
		Status:          "imported_fixture",
		ReadingReviewFlags: []ReadingReviewFlag{
			{
				Flag:   defaultReviewFlag,
				Scope:  "all learner_kana, readings, romaji_hint, meaning_hint, prerequisite_cards, and learner_hints fields",
				Reason: "Importer output preserves official text and marks learner-layer readings and hints for review before they are treated as playable content.",
			},
			{
				Flag:   "official_text_preserved",
				Scope:  "document.sections[].official_text",
				Reason: "Importer does not modernize source orthography or punctuation; learner-facing normalization belongs in campaign hints.",
			},
		},
		Levels: levels,
	}

	return &Output{Document: document, Campaign: campaign}, nil
}

func WriteJSON(w io.Writer, value any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(value)
}

func validateInputPath(path string) error {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".md", ".txt":
		return nil
	default:
		return fmt.Errorf("unsupported input extension %q: expected .md or .txt", ext)
	}
}

func splitSections(source string) ([]parsedSection, string) {
	lines := splitRawLines(source)
	var sections []parsedSection
	var current *currentSection
	var title string
	var chapterNumber int
	var chapterTitle string
	seenContent := false

	flush := func() {
		if current == nil {
			return
		}
		sectionLines := trimBlankLines(current.lines)
		if len(sectionLines) == 0 {
			current = nil
			return
		}
		text := joinRawLines(sectionLines)
		body := text
		if current.includeHeading && current.articleNumber > 0 {
			body = strings.TrimSpace(articleHeadingRE.ReplaceAllString(sectionLines[0].text, "$2"))
			if len(sectionLines) > 1 {
				rest := joinRawLines(sectionLines[1:])
				body = strings.TrimSpace(strings.Join(nonBlank(body, rest), "\n"))
			}
		}
		sections = append(sections, parsedSection{
			heading:         current.heading,
			kind:            current.kind,
			text:            text,
			body:            body,
			articleNumber:   current.articleNumber,
			chapterNumber:   current.chapterNumber,
			chapterTitleJA:  current.chapterTitleJA,
			includeHeading:  current.includeHeading,
			markdownSection: current.markdownSection,
		})
		current = nil
	}

	for i, line := range lines {
		classified := classifyHeading(line.text)
		if classified.ok {
			if classified.docTitle {
				if title == "" {
					title = classified.title
				}
				seenContent = true
				continue
			}

			if classified.chapter {
				flush()
				chapterNumber = classified.chapterNumber
				chapterTitle = classified.title
				seenContent = true
				continue
			}

			if classified.preamble || classified.article || classified.genericSection {
				flush()
				kind := "section"
				if classified.preamble {
					kind = "preamble"
				}
				if classified.article {
					kind = "article"
				}
				current = &currentSection{
					heading:         classified.title,
					kind:            kind,
					articleNumber:   classified.articleNumber,
					chapterNumber:   chapterNumber,
					chapterTitleJA:  chapterTitle,
					includeHeading:  classified.includeLine,
					markdownSection: classified.markdown,
				}
				if classified.includeLine {
					current.lines = append(current.lines, line)
				} else if strings.TrimSpace(classified.body) != "" {
					current.lines = append(current.lines, rawLine{text: classified.body, newline: line.newline})
				}
				seenContent = true
				continue
			}
		}

		if current == nil && !seenContent && strings.TrimSpace(line.text) != "" && looksLikePlainTitle(i, lines) {
			title = strings.TrimSpace(line.text)
			seenContent = true
			continue
		}

		if current == nil {
			current = &currentSection{
				heading: "Body",
				kind:    "section",
			}
		}
		current.lines = append(current.lines, line)
		if strings.TrimSpace(line.text) != "" {
			seenContent = true
		}
	}
	flush()

	return expandPreambleSections(sections), title
}

func classifyHeading(line string) heading {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return heading{}
	}

	if strings.HasPrefix(trimmed, "#") {
		level, title := parseMarkdownHeading(trimmed)
		if level == 1 {
			return heading{ok: true, docTitle: true, title: title, markdown: true}
		}
		if level >= 2 {
			classified := classifyPlainHeading(title)
			classified.ok = true
			classified.markdown = true
			classified.includeLine = false
			if !classified.preamble && !classified.article && !classified.chapter {
				classified.genericSection = true
				classified.title = title
			}
			return classified
		}
	}

	return classifyPlainHeading(trimmed)
}

func classifyPlainHeading(trimmed string) heading {
	if trimmed == "前文" {
		return heading{ok: true, preamble: true, title: "前文"}
	}
	if matches := chapterHeadingRE.FindStringSubmatch(trimmed); matches != nil {
		number := parseJapaneseNumber(matches[1])
		return heading{
			ok:            true,
			chapter:       true,
			title:         strings.TrimSpace(matches[2]),
			chapterNumber: number,
		}
	}
	if matches := articleHeadingRE.FindStringSubmatch(trimmed); matches != nil {
		number := parseJapaneseNumber(matches[1])
		body := strings.TrimSpace(matches[2])
		return heading{
			ok:            true,
			article:       true,
			title:         "第" + matches[1] + "条",
			articleNumber: number,
			body:          body,
			includeLine:   true,
		}
	}
	return heading{}
}

func parseMarkdownHeading(line string) (int, string) {
	level := 0
	for level < len(line) && line[level] == '#' {
		level++
	}
	if level == 0 || level == len(line) {
		return 0, ""
	}
	if line[level] != ' ' && line[level] != '\t' {
		return 0, ""
	}
	return level, strings.TrimSpace(line[level:])
}

func looksLikePlainTitle(index int, lines []rawLine) bool {
	for i := index + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i].text) == "" {
			continue
		}
		classified := classifyPlainHeading(strings.TrimSpace(lines[i].text))
		return classified.ok
	}
	return false
}

func expandPreambleSections(sections []parsedSection) []parsedSection {
	var expanded []parsedSection
	for _, section := range sections {
		if section.kind != "preamble" {
			expanded = append(expanded, section)
			continue
		}
		paragraphs := splitParagraphs(section.text)
		if len(paragraphs) == 0 {
			continue
		}
		for i, paragraph := range paragraphs {
			expanded = append(expanded, parsedSection{
				heading:        fmt.Sprintf("前文 第%d段", i+1),
				kind:           "preamble_paragraph",
				text:           paragraph,
				body:           paragraph,
				chapterNumber:  section.chapterNumber,
				chapterTitleJA: section.chapterTitleJA,
			})
		}
	}
	return expanded
}

func splitParagraphs(text string) []string {
	lines := splitRawLines(text)
	var paragraphs []string
	var current []rawLine
	flush := func() {
		current = trimBlankLines(current)
		if len(current) == 0 {
			return
		}
		paragraphs = append(paragraphs, joinRawLines(current))
		current = nil
	}
	for _, line := range lines {
		if strings.TrimSpace(line.text) == "" {
			flush()
			continue
		}
		current = append(current, line)
	}
	flush()
	return paragraphs
}

func toDocumentSection(section parsedSection, used map[string]int) DocumentSection {
	id := sectionID(section, used)
	label := LocalizedTitle{JA: section.heading}
	if label.JA == "" {
		label.JA = id
	}

	documentSection := DocumentSection{
		SectionID:    id,
		Kind:         firstNonBlank(section.kind, "section"),
		Label:        label,
		OfficialText: section.text,
	}
	if section.kind == "article" {
		documentSection.Article = &ArticleInfo{
			Number:  section.articleNumber,
			TitleJA: firstNonBlank(section.heading, fmt.Sprintf("第%d条", section.articleNumber)),
		}
		documentSection.OfficialBody = section.body
		if section.chapterNumber > 0 {
			documentSection.Chapter = &ChapterInfo{
				Number:  section.chapterNumber,
				TitleJA: section.chapterTitleJA,
			}
			if section.chapterTitleJA != "" {
				documentSection.Label.JA = fmt.Sprintf("第%d章 %s %s", section.chapterNumber, section.chapterTitleJA, documentSection.Article.TitleJA)
			}
		}
	}
	return documentSection
}

func sectionID(section parsedSection, used map[string]int) string {
	var id string
	switch section.kind {
	case "preamble_paragraph":
		id = "preamble-" + strconv.Itoa(used["preamble"]+1)
		used["preamble"]++
	case "article":
		if section.articleNumber > 0 {
			id = "article-" + strconv.Itoa(section.articleNumber)
		}
	default:
		id = slug(section.heading)
	}
	if id == "" {
		id = "section"
	}
	used[id]++
	if used[id] > 1 {
		return id + "-" + strconv.Itoa(used[id])
	}
	return id
}

func splitRawLines(source string) []rawLine {
	if source == "" {
		return nil
	}
	parts := strings.SplitAfter(source, "\n")
	if len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	lines := make([]rawLine, 0, len(parts))
	for _, part := range parts {
		line := rawLine{text: part}
		if strings.HasSuffix(line.text, "\n") {
			line.newline = "\n"
			line.text = strings.TrimSuffix(line.text, "\n")
			if strings.HasSuffix(line.text, "\r") {
				line.text = strings.TrimSuffix(line.text, "\r")
				line.newline = "\r\n"
			}
		}
		lines = append(lines, line)
	}
	return lines
}

func trimBlankLines(lines []rawLine) []rawLine {
	start := 0
	for start < len(lines) && strings.TrimSpace(lines[start].text) == "" {
		start++
	}
	end := len(lines)
	for end > start && strings.TrimSpace(lines[end-1].text) == "" {
		end--
	}
	return lines[start:end]
}

func joinRawLines(lines []rawLine) string {
	var b strings.Builder
	for i, line := range lines {
		b.WriteString(line.text)
		if i < len(lines)-1 {
			b.WriteString(line.newline)
			if line.newline == "" {
				b.WriteByte('\n')
			}
		}
	}
	return b.String()
}

func parseJapaneseNumber(value string) int {
	if value == "" {
		return 0
	}
	if n, err := strconv.Atoi(value); err == nil {
		return n
	}

	digits := map[rune]int{
		'〇': 0,
		'零': 0,
		'一': 1,
		'二': 2,
		'三': 3,
		'四': 4,
		'五': 5,
		'六': 6,
		'七': 7,
		'八': 8,
		'九': 9,
	}
	units := map[rune]int{
		'十': 10,
		'百': 100,
		'千': 1000,
	}

	total := 0
	current := 0
	for _, r := range value {
		if digit, ok := digits[r]; ok {
			current = digit
			continue
		}
		unit, ok := units[r]
		if !ok {
			return 0
		}
		if current == 0 {
			current = 1
		}
		total += current * unit
		current = 0
	}
	return total + current
}

func slugFromPath(path string) string {
	if path == "" {
		return ""
	}
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	return slug(strings.TrimSuffix(base, ext))
}

func slug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastHyphen := false
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			lastHyphen = false
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			lastHyphen = false
		case unicode.IsLetter(r) || unicode.IsNumber(r):
			b.WriteRune(r)
			lastHyphen = false
		default:
			if !lastHyphen && b.Len() > 0 {
				b.WriteByte('-')
				lastHyphen = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func nonBlank(values ...string) []string {
	var out []string
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			out = append(out, strings.TrimSpace(value))
		}
	}
	return out
}
