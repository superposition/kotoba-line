package content

type CardType string

const (
	CardTypeKanjiReading CardType = "kanji_reading"
	CardTypeWord         CardType = "word"
	CardTypeDate         CardType = "date"
	CardTypeTimeWord     CardType = "time_word"
	CardTypeWeekday      CardType = "weekday"
	CardTypePlaceName    CardType = "place_name"
	CardTypePhrase       CardType = "phrase"
	CardTypeKanaHiragana CardType = "kana_hiragana"
	CardTypeKanaKatakana CardType = "kana_katakana"
	CardTypeKanaCompare  CardType = "kana_comparison"
)

type Library struct {
	Cards     []Card     `json:"cards"`
	Documents []Document `json:"documents"`
	Levels    []Level    `json:"levels"`
	Campaigns []Campaign `json:"campaigns"`
}

type Card struct {
	ID          string   `json:"id"`
	Text        string   `json:"text"`
	Kanji       string   `json:"kanji,omitempty"`
	Reading     Reading  `json:"reading"`
	Meaning     string   `json:"meaning"`
	Type        CardType `json:"type"`
	Playable    bool     `json:"playable"`
	NeedsReview bool     `json:"needs_review"`
	SourceRef   string   `json:"source_ref,omitempty"`
	Notes       []string `json:"notes,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

type Reading struct {
	Kana       string `json:"kana"`
	RomajiHint string `json:"romaji_hint"`
}

type Document struct {
	ID         string            `json:"id"`
	Title      string            `json:"title"`
	SourcePath string            `json:"source_path,omitempty"`
	SourceDate string            `json:"source_date,omitempty"`
	Sections   []DocumentSection `json:"sections"`
}

type DocumentSection struct {
	ID       string `json:"id"`
	Heading  string `json:"heading"`
	Markdown string `json:"markdown"`
}

type Level struct {
	ID              string   `json:"id"`
	Title           string   `json:"title"`
	Description     string   `json:"description,omitempty"`
	DocumentID      string   `json:"document_id,omitempty"`
	SectionID       string   `json:"section_id,omitempty"`
	CardIDs         []string `json:"card_ids"`
	RequiredCardIDs []string `json:"required_card_ids,omitempty"`
	RequiredPoints  int      `json:"required_points,omitempty"`
}

type Campaign struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	Description  string   `json:"description,omitempty"`
	DocumentIDs  []string `json:"document_ids"`
	LevelIDs     []string `json:"level_ids"`
	StartLevelID string   `json:"start_level_id,omitempty"`
}
