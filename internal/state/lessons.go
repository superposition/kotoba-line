package state

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/superposition/kotoba-line/internal/content"
	"github.com/superposition/kotoba-line/internal/kana"
)

type lessonSeed struct {
	ID               string
	Title            string
	Description      string
	DocumentTitle    string
	RequiredLessonID string
	RequiredPoints   int
	Cards            []lessonCardSeed
}

type lessonCardSeed struct {
	ID         string
	Text       string
	Kanji      string
	Kana       string
	RomajiHint string
	Meaning    string
	Type       content.CardType
	Notes      string
	Tags       string
}

func SeedDefaultLessons(path string) (int, error) {
	store := NewSQLiteEventStore(path, "system")
	db, err := store.open()
	if err != nil {
		return 0, err
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin lesson seed: %w", err)
	}
	defer tx.Rollback()

	inserted := 0
	for lessonIndex, lesson := range defaultLessonSeeds() {
		if _, err := tx.Exec(`
			INSERT INTO lessons (id, position, title, description, document_title, required_lesson_id, required_points)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				position = excluded.position,
				title = excluded.title,
				description = excluded.description,
				document_title = excluded.document_title,
				required_lesson_id = excluded.required_lesson_id,
				required_points = excluded.required_points
		`, lesson.ID, lessonIndex, lesson.Title, lesson.Description, lesson.DocumentTitle, lesson.RequiredLessonID, lesson.RequiredPoints); err != nil {
			return 0, fmt.Errorf("seed lesson %s: %w", lesson.ID, err)
		}
		for cardIndex, card := range lesson.Cards {
			result, err := tx.Exec(`
				INSERT INTO lesson_cards (id, lesson_id, position, text, kanji, kana, romaji_hint, meaning, type, notes, tags, playable)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1)
				ON CONFLICT(id) DO UPDATE SET
					lesson_id = excluded.lesson_id,
					position = excluded.position,
					text = excluded.text,
					kanji = excluded.kanji,
					kana = excluded.kana,
					romaji_hint = excluded.romaji_hint,
					meaning = excluded.meaning,
					type = excluded.type,
					notes = excluded.notes,
					tags = excluded.tags,
					playable = excluded.playable
			`, card.ID, lesson.ID, cardIndex, card.Text, firstNonBlank(card.Kanji, card.Text), card.Kana, card.RomajiHint, card.Meaning, string(card.Type), card.Notes, card.Tags)
			if err != nil {
				return 0, fmt.Errorf("seed lesson card %s: %w", card.ID, err)
			}
			if affected, _ := result.RowsAffected(); affected > 0 {
				inserted++
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit lesson seed: %w", err)
	}
	return inserted, nil
}

func LoadLessonLibrary(path string) (*content.Library, content.ValidationReport, error) {
	store := NewSQLiteEventStore(path, "system")
	db, err := store.open()
	if err != nil {
		return nil, content.ValidationReport{}, err
	}
	defer db.Close()

	lessons, err := readLessons(db)
	if err != nil {
		return nil, content.ValidationReport{}, err
	}
	if len(lessons) == 0 {
		report := content.ValidationReport{}
		return &content.Library{}, report, nil
	}

	library := &content.Library{
		Documents: []content.Document{{
			ID:    "sqlite-hi-foundation",
			Title: lessons[0].DocumentTitle,
		}},
		Campaigns: []content.Campaign{{
			ID:           "sqlite-hi-foundation",
			Title:        "Kana And 日 Foundation",
			Description:  "SQLite-backed kana comparison lessons before 日, 日本, dates, and core sentences.",
			DocumentIDs:  []string{"sqlite-hi-foundation"},
			StartLevelID: lessons[0].ID,
		}},
	}
	cardIDsByLesson := map[string][]string{}
	for _, lesson := range lessons {
		cards, err := readLessonCards(db, lesson.ID)
		if err != nil {
			return nil, content.ValidationReport{}, err
		}
		cardIDs := make([]string, 0, len(cards))
		for _, card := range cards {
			library.Cards = append(library.Cards, card)
			cardIDs = append(cardIDs, card.ID)
		}
		cardIDsByLesson[lesson.ID] = append([]string(nil), cardIDs...)
		level := content.Level{
			ID:             lesson.ID,
			Title:          lesson.Title,
			Description:    lesson.Description,
			DocumentID:     "sqlite-hi-foundation",
			SectionID:      lesson.ID,
			CardIDs:        cardIDs,
			RequiredPoints: lesson.RequiredPoints,
		}
		if lesson.RequiredLessonID != "" {
			level.RequiredCardIDs = append([]string(nil), cardIDsByLesson[lesson.RequiredLessonID]...)
		}
		library.Levels = append(library.Levels, level)
		library.Documents[0].Sections = append(library.Documents[0].Sections, content.DocumentSection{
			ID:       lesson.ID,
			Heading:  lesson.Title,
			Markdown: lesson.Description,
		})
		library.Campaigns[0].LevelIDs = append(library.Campaigns[0].LevelIDs, lesson.ID)
	}
	report := content.ValidateLibrary(library)
	return library, report, nil
}

func DefaultLessonLibrary() (*content.Library, content.ValidationReport) {
	library := &content.Library{
		Documents: []content.Document{{
			ID:    "hi-foundation",
			Title: "Kana And 日 Foundation",
		}},
		Campaigns: []content.Campaign{{
			ID:           "hi-foundation",
			Title:        "Kana And 日 Foundation",
			Description:  "Focused kana comparison lessons before 日, 日本, dates, and core sentences.",
			DocumentIDs:  []string{"hi-foundation"},
			StartLevelID: "lesson-kana-hiragana-early",
		}},
	}

	cardIDsByLesson := map[string][]string{}
	for _, lesson := range defaultLessonSeeds() {
		cardIDs := make([]string, 0, len(lesson.Cards))
		for _, seeded := range lesson.Cards {
			card := content.Card{
				ID:        seeded.ID,
				Text:      seeded.Text,
				Kanji:     firstNonBlank(seeded.Kanji, seeded.Text),
				Reading:   content.Reading{Kana: seeded.Kana, RomajiHint: seeded.RomajiHint},
				Meaning:   seeded.Meaning,
				Type:      seeded.Type,
				Playable:  true,
				Notes:     splitPipes(seeded.Notes),
				Tags:      splitPipes(seeded.Tags),
				SourceRef: "built-in:lesson-seed",
			}
			library.Cards = append(library.Cards, card)
			cardIDs = append(cardIDs, card.ID)
		}
		cardIDsByLesson[lesson.ID] = append([]string(nil), cardIDs...)
		level := content.Level{
			ID:             lesson.ID,
			Title:          lesson.Title,
			Description:    lesson.Description,
			DocumentID:     "hi-foundation",
			SectionID:      lesson.ID,
			CardIDs:        cardIDs,
			RequiredPoints: lesson.RequiredPoints,
		}
		if lesson.RequiredLessonID != "" {
			level.RequiredCardIDs = append([]string(nil), cardIDsByLesson[lesson.RequiredLessonID]...)
		}
		library.Levels = append(library.Levels, level)
		library.Documents[0].Sections = append(library.Documents[0].Sections, content.DocumentSection{
			ID:       lesson.ID,
			Heading:  lesson.Title,
			Markdown: lesson.Description,
		})
		library.Campaigns[0].LevelIDs = append(library.Campaigns[0].LevelIDs, lesson.ID)
	}
	return library, content.ValidateLibrary(library)
}

type dbLesson struct {
	ID               string
	Title            string
	Description      string
	DocumentTitle    string
	RequiredLessonID string
	RequiredPoints   int
}

func readLessons(db *sql.DB) ([]dbLesson, error) {
	rows, err := db.Query(`
		SELECT id, title, description, document_title, required_lesson_id, required_points
		FROM lessons
		ORDER BY position ASC, id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("read lessons: %w", err)
	}
	defer rows.Close()

	var lessons []dbLesson
	for rows.Next() {
		var lesson dbLesson
		if err := rows.Scan(&lesson.ID, &lesson.Title, &lesson.Description, &lesson.DocumentTitle, &lesson.RequiredLessonID, &lesson.RequiredPoints); err != nil {
			return nil, fmt.Errorf("scan lesson: %w", err)
		}
		lessons = append(lessons, lesson)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate lessons: %w", err)
	}
	return lessons, nil
}

func readLessonCards(db *sql.DB, lessonID string) ([]content.Card, error) {
	rows, err := db.Query(`
		SELECT id, text, kanji, kana, romaji_hint, meaning, type, notes, tags, playable
		FROM lesson_cards
		WHERE lesson_id = ?
		ORDER BY position ASC, id ASC
	`, lessonID)
	if err != nil {
		return nil, fmt.Errorf("read lesson cards: %w", err)
	}
	defer rows.Close()

	var cards []content.Card
	for rows.Next() {
		var card content.Card
		var cardType string
		var notes string
		var tags string
		var playable int
		if err := rows.Scan(&card.ID, &card.Text, &card.Kanji, &card.Reading.Kana, &card.Reading.RomajiHint, &card.Meaning, &cardType, &notes, &tags, &playable); err != nil {
			return nil, fmt.Errorf("scan lesson card: %w", err)
		}
		card.Type = content.CardType(cardType)
		card.Playable = playable != 0
		card.Notes = splitPipes(notes)
		card.Tags = splitPipes(tags)
		card.SourceRef = "sqlite:lesson_cards"
		cards = append(cards, card)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate lesson cards: %w", err)
	}
	return cards, nil
}

func splitPipes(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, "|")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

const kanaFoundationFinalLevelID = "lesson-kana-comparison"

func kanaFoundationLessonSeeds() []lessonSeed {
	return []lessonSeed{
		{
			ID:            "lesson-kana-hiragana-early",
			Title:         "Kana 1 - Hiragana A/K/S/T/N",
			Description:   "Build hiragana from row plus vowel coordinates before reading kanji.",
			DocumentTitle: "Kana And 日 Foundation",
			Cards:         kanaFoundationCards("lesson-kana-hira", kana.BasicRows()[:5], kana.ScriptHiragana),
		},
		{
			ID:               "lesson-kana-hiragana-late",
			Title:            "Kana 2 - Hiragana H/M/Y/R/W/N",
			Description:      "Finish the base hiragana map, including sparse y/w rows and ん.",
			DocumentTitle:    "Kana And 日 Foundation",
			RequiredLessonID: "lesson-kana-hiragana-early",
			Cards:            kanaFoundationCards("lesson-kana-hira", kana.BasicRows()[5:], kana.ScriptHiragana),
		},
		{
			ID:               "lesson-kana-katakana-early",
			Title:            "Kana 3 - Katakana A/K/S/T/N",
			Description:      "Map the same row and vowel coordinates into katakana.",
			DocumentTitle:    "Kana And 日 Foundation",
			RequiredLessonID: "lesson-kana-hiragana-late",
			Cards:            kanaFoundationCards("lesson-kana-kata", kana.BasicRows()[:5], kana.ScriptKatakana),
		},
		{
			ID:               "lesson-kana-katakana-late",
			Title:            "Kana 4 - Katakana H/M/Y/R/W/N",
			Description:      "Finish base katakana and keep comparing it against hiragana.",
			DocumentTitle:    "Kana And 日 Foundation",
			RequiredLessonID: "lesson-kana-katakana-early",
			Cards:            kanaFoundationCards("lesson-kana-kata", kana.BasicRows()[5:], kana.ScriptKatakana),
		},
		{
			ID:               kanaFoundationFinalLevelID,
			Title:            "Kana 5 - Script Comparisons",
			Description:      "Beat the lookalike traps by choosing the exact hiragana or katakana form.",
			DocumentTitle:    "Kana And 日 Foundation",
			RequiredLessonID: "lesson-kana-katakana-late",
			Cards:            kanaComparisonCards(),
		},
	}
}

func kanaFoundationCards(idPrefix string, rows []kana.Row, script kana.Script) []lessonCardSeed {
	cards := make([]lessonCardSeed, 0)
	for _, row := range rows {
		for _, cell := range row.Cells {
			answer := cell.Hiragana
			companion := cell.Katakana
			cardType := content.CardTypeKanaHiragana
			scriptName := "hiragana"
			if script == kana.ScriptKatakana {
				answer = cell.Katakana
				companion = cell.Hiragana
				cardType = content.CardTypeKanaKatakana
				scriptName = "katakana"
			}

			text := fmt.Sprintf("%s + %s / %s", row.Label, firstNonBlank(cell.Vowel, cell.Romaji), companion)
			if cell.Romaji == "n" {
				text = fmt.Sprintf("n / %s", companion)
			}
			cards = append(cards, lessonCardSeed{
				ID:         fmt.Sprintf("%s-%s", idPrefix, kanaCardID(cell.Romaji)),
				Text:       text,
				Kanji:      text,
				Kana:       answer,
				RomajiHint: cell.Romaji,
				Meaning:    fmt.Sprintf("%s %s", scriptName, cell.Romaji),
				Type:       cardType,
				Notes:      fmt.Sprintf("kana row %s|vowel %s|compare %s", row.Label, firstNonBlank(cell.Vowel, "n"), companion),
				Tags:       fmt.Sprintf("sqlite|kana-foundation|%s|row-%s", scriptName, row.Label),
			})
		}
	}
	return cards
}

func kanaCardID(romaji string) string {
	return strings.NewReplacer("'", "n", " ", "-", "_", "-").Replace(romaji)
}

func kanaComparisonCards() []lessonCardSeed {
	type comparison struct {
		ID      string
		Text    string
		Kana    string
		Romaji  string
		Meaning string
	}
	comparisons := []comparison{
		{"kata-shi-vs-tsu", "shi  シ / ツ", "シ", "shi", "katakana shi, not tsu"},
		{"kata-tsu-vs-shi", "tsu  ツ / シ", "ツ", "tsu", "katakana tsu, not shi"},
		{"kata-so-vs-n", "so  ソ / ン", "ソ", "so", "katakana so, not n"},
		{"kata-n-vs-so", "n  ン / ソ", "ン", "n", "katakana n, not so"},
		{"hira-nu-vs-me", "nu  ぬ / め", "ぬ", "nu", "hiragana nu, not me"},
		{"hira-me-vs-nu", "me  め / ぬ", "め", "me", "hiragana me, not nu"},
		{"hira-ne-vs-re", "ne  ね / れ", "ね", "ne", "hiragana ne, not re"},
		{"hira-re-vs-ne", "re  れ / ね", "れ", "re", "hiragana re, not ne"},
		{"hira-wa-vs-re", "wa  わ / れ", "わ", "wa", "hiragana wa, not re"},
		{"hira-ha-vs-ho", "ha  は / ほ", "は", "ha", "hiragana ha, not ho"},
		{"hira-ho-vs-ha", "ho  ほ / は", "ほ", "ho", "hiragana ho, not ha"},
		{"hira-ru-vs-ro", "ru  る / ろ", "る", "ru", "hiragana ru, not ro"},
		{"hira-ro-vs-ru", "ro  ろ / る", "ろ", "ro", "hiragana ro, not ru"},
		{"kata-a-vs-hira-a", "a  ア / あ", "ア", "a", "katakana a, not hiragana a"},
	}
	cards := make([]lessonCardSeed, 0, len(comparisons))
	for _, item := range comparisons {
		cards = append(cards, lessonCardSeed{
			ID:         "lesson-kana-compare-" + item.ID,
			Text:       item.Text,
			Kanji:      item.Text,
			Kana:       item.Kana,
			RomajiHint: item.Romaji,
			Meaning:    item.Meaning,
			Type:       content.CardTypeKanaCompare,
			Notes:      "kana comparison|lookalike trap",
			Tags:       "sqlite|kana-foundation|comparison",
		})
	}
	return cards
}

func defaultLessonSeeds() []lessonSeed {
	lessons := append(kanaFoundationLessonSeeds(), []lessonSeed{
		{
			ID:               "lesson-hi-readings",
			Title:            "Lesson 1 - 日 Readings",
			Description:      "Stay on 日 until the core readings are clean: ひ, にち, び, and か.",
			DocumentTitle:    "SQLite Lesson - 日 Foundation",
			RequiredLessonID: kanaFoundationFinalLevelID,
			Cards: []lessonCardSeed{
				cardSeed("hi", "日", "ひ", "hi", "sun; day", content.CardTypeKanjiReading, "standalone 日, as in 日が暮れる"),
				cardSeed("nichi", "日", "にち", "nichi", "day; date; Japan-word reading", content.CardTypeKanjiReading, "common on-yomi in 日本 and 日曜日"),
				cardSeed("bi", "日", "び", "bi", "day in weekday words", content.CardTypeKanjiReading, "sound change in words like 日曜日"),
				cardSeed("ka", "日", "か", "ka", "date counter reading", content.CardTypeKanjiReading, "used in many calendar dates"),
			},
		},
		{
			ID:               "lesson-hi-words",
			Title:            "Lesson 2 - 日 Words And Dates",
			Description:      "Only 日-family words and date forms. No new topic until these are stable.",
			DocumentTitle:    "SQLite Lesson - 日 Foundation",
			RequiredLessonID: "lesson-hi-readings",
			Cards: []lessonCardSeed{
				cardSeed("nihon", "日本", "にほん", "nihon", "Japan", content.CardTypeWord, "everyday reading"),
				cardSeed("nippon", "日本", "にっぽん", "nippon", "Nippon; formal/emphatic Japan", content.CardTypeWord, "special reading; useful for names and formal contexts"),
				cardSeed("honjitsu", "本日", "ほんじつ", "honjitsu", "today; formal", content.CardTypeTimeWord, "formal version of today"),
				cardSeed("mainichi", "毎日", "まいにち", "mainichi", "every day", content.CardTypeTimeWord, "common daily word"),
				cardSeed("tsuitachi", "1日", "ついたち", "tsuitachi", "first day of the month", content.CardTypeDate, "special date"),
				cardSeed("futsuka", "2日", "ふつか", "futsuka", "second day; two days", content.CardTypeDate, "special date"),
				cardSeed("mikka", "3日", "みっか", "mikka", "third day; three days", content.CardTypeDate, "special date"),
				cardSeed("yokka", "4日", "よっか", "yokka", "fourth day; four days", content.CardTypeDate, "special date"),
				cardSeed("itsuka", "5日", "いつか", "itsuka", "fifth day; five days", content.CardTypeDate, "special date"),
				cardSeed("nanoka", "7日", "なのか", "nanoka", "seventh day; seven days", content.CardTypeDate, "special date"),
				cardSeed("tooka", "10日", "とおか", "tooka", "tenth day; ten days", content.CardTypeDate, "special date"),
				cardSeed("hatsuka", "20日", "はつか", "hatsuka", "twentieth day; twenty days", content.CardTypeDate, "special date"),
			},
		},
		{
			ID:               "lesson-hi-sentences",
			Title:            "Lesson 3 - 日 Sentences",
			Description:      "Sentences from the whiteboard, still locked to 日-family reading practice.",
			DocumentTitle:    "SQLite Lesson - 日 Foundation",
			RequiredLessonID: "lesson-hi-words",
			Cards: []lessonCardSeed{
				cardSeed("higa-kureru", "日が暮れる", "ひがくれる", "hi ga kureru", "the sun sets", content.CardTypePhrase, "日 as ひ"),
				cardSeed("nippori-mainichi", "日暮里で毎日日が暮れる", "にっぽりでまいにちひがくれる", "nippori de mainichi hi ga kureru", "In Nippori, the sun sets every day.", content.CardTypePhrase, "日暮里 uses にっぽり; 毎日 uses まいにち; 日が uses ひ"),
				cardSeed("kasugacho-higure", "春日町でも日が暮れる", "かすがちょうでもひがくれる", "kasugachou demo hi ga kureru", "The sun sets in Kasugacho too.", content.CardTypePhrase, "日 in 春日町 is part of a place name"),
				cardSeed("honjitsu-thanks", "本日は誠にありがとうございました", "ほんじつはまことにありがとうございました", "honjitsu wa makoto ni arigatou gozaimashita", "Thank you very much today.", content.CardTypePhrase, "formal 本日 sentence"),
			},
		},
		{
			ID:               "lesson-hi-particles",
			Title:            "Lesson 4 - Particles And Glue",
			Description:      "Small grammar pieces that make the sentence tree readable: topic, subject, place, direction, and too/also.",
			DocumentTitle:    "SQLite Lesson - 日 Foundation",
			RequiredLessonID: "lesson-hi-sentences",
			RequiredPoints:   1200,
			Cards: []lessonCardSeed{
				cardSeed("particle-wa", "は", "は", "wa", "topic marker", content.CardTypeWord, "read as わ when it marks the topic"),
				cardSeed("particle-ga", "が", "が", "ga", "subject marker", content.CardTypeWord, "marks the subject doing or being something"),
				cardSeed("particle-de", "で", "で", "de", "place of action", content.CardTypeWord, "marks where an action happens"),
				cardSeed("particle-ni", "に", "に", "ni", "direction; time; target", content.CardTypeWord, "points to a destination, time, or target"),
				cardSeed("particle-demo", "でも", "でも", "demo", "also; even at", content.CardTypeWord, "combines で + も for also/even in a place"),
			},
		},
		{
			ID:               "lesson-hi-patterns",
			Title:            "Lesson 5 - Seaside Sentence Patterns",
			Description:      "Short sentence trees that combine the learned 日 words with place, time, topic, and polite verbs.",
			DocumentTitle:    "SQLite Lesson - 日 Foundation",
			RequiredLessonID: "lesson-hi-particles",
			RequiredPoints:   2200,
			Cards: []lessonCardSeed{
				cardSeed("nihon-mainichi-benkyou", "日本で毎日勉強します", "にほんでまいにちべんきょうします", "nihon de mainichi benkyou shimasu", "I study in Japan every day.", content.CardTypePhrase, "日本で = in Japan; 毎日 = every day; 勉強します = study politely"),
				cardSeed("honjitsu-nihon-benkyou", "本日は日本で勉強しました", "ほんじつはにほんでべんきょうしました", "honjitsu wa nihon de benkyou shimashita", "Today, I studied in Japan.", content.CardTypePhrase, "本日は = formal today topic; 日本で = in Japan; 勉強しました = studied politely"),
				cardSeed("nippori-demo-benkyou", "日暮里でも勉強しました", "にっぽりでもべんきょうしました", "nippori demo benkyou shimashita", "I studied in Nippori too.", content.CardTypePhrase, "日暮里でも = in Nippori too; 勉強しました = studied politely"),
			},
		},
		{
			ID:               "lesson-hi-n5-verbs",
			Title:            "Lesson 6 - N5 Action Verbs",
			Description:      "Core polite verbs used to build fast N5 sentences after the 日 foundation.",
			DocumentTitle:    "SQLite Lesson - 日 Foundation",
			RequiredLessonID: "lesson-hi-patterns",
			RequiredPoints:   3200,
			Cards: []lessonCardSeed{
				cardSeed("ikimasu", "行きます", "いきます", "ikimasu", "go", content.CardTypeWord, "polite non-past verb"),
				cardSeed("kimasu", "来ます", "きます", "kimasu", "come", content.CardTypeWord, "polite non-past verb"),
				cardSeed("kaerimasu", "帰ります", "かえります", "kaerimasu", "return home", content.CardTypeWord, "polite non-past verb"),
				cardSeed("tabemasu", "食べます", "たべます", "tabemasu", "eat", content.CardTypeWord, "polite non-past verb"),
				cardSeed("nomimasu", "飲みます", "のみます", "nomimasu", "drink", content.CardTypeWord, "polite non-past verb"),
				cardSeed("mimasu", "見ます", "みます", "mimasu", "see; watch", content.CardTypeWord, "polite non-past verb"),
				cardSeed("kikimasu", "聞きます", "ききます", "kikimasu", "listen; ask", content.CardTypeWord, "polite non-past verb"),
				cardSeed("yomimasu", "読みます", "よみます", "yomimasu", "read", content.CardTypeWord, "polite non-past verb"),
			},
		},
		{
			ID:               "lesson-hi-n5-anchors",
			Title:            "Lesson 7 - N5 People Places Objects",
			Description:      "Nouns that let the verb wave turn into real beginner sentences.",
			DocumentTitle:    "SQLite Lesson - 日 Foundation",
			RequiredLessonID: "lesson-hi-n5-verbs",
			RequiredPoints:   4200,
			Cards: []lessonCardSeed{
				cardSeed("gakkou", "学校", "がっこう", "gakkou", "school", content.CardTypeWord, "place noun"),
				cardSeed("ie", "家", "いえ", "ie", "house; home", content.CardTypeWord, "place noun"),
				cardSeed("sensei", "先生", "せんせい", "sensei", "teacher", content.CardTypeWord, "person noun"),
				cardSeed("gakusei", "学生", "がくせい", "gakusei", "student", content.CardTypeWord, "person noun"),
				cardSeed("tomodachi", "友達", "ともだち", "tomodachi", "friend", content.CardTypeWord, "person noun"),
				cardSeed("hon", "本", "ほん", "hon", "book", content.CardTypeWord, "object noun"),
				cardSeed("mizu", "水", "みず", "mizu", "water", content.CardTypeWord, "object noun"),
				cardSeed("eiga", "映画", "えいが", "eiga", "movie", content.CardTypeWord, "object noun"),
			},
		},
		{
			ID:               "lesson-hi-n5-sentence-trees",
			Title:            "Lesson 8 - N5 Sentence Trees",
			Description:      "Beginner sentences that make topic, time, destination, object, and companion roles visible.",
			DocumentTitle:    "SQLite Lesson - 日 Foundation",
			RequiredLessonID: "lesson-hi-n5-anchors",
			RequiredPoints:   5200,
			Cards: []lessonCardSeed{
				cardSeed("kyou-gakkou-ikimasu", "今日は学校へ行きます", "きょうはがっこうへいきます", "kyou wa gakkou e ikimasu", "Today I go to school.", content.CardTypePhrase, "今日 = today; 学校へ = to school; 行きます = go"),
				cardSeed("ashita-nihon-ikimasu", "明日は日本へ行きます", "あしたはにほんへいきます", "ashita wa nihon e ikimasu", "Tomorrow I go to Japan.", content.CardTypePhrase, "明日 = tomorrow; 日本へ = to Japan; 行きます = go"),
				cardSeed("kinou-ie-kaerimashita", "昨日は家へ帰りました", "きのうはいえへかえりました", "kinou wa ie e kaerimashita", "Yesterday I returned home.", content.CardTypePhrase, "昨日 = yesterday; 家へ = homeward; 帰りました = returned"),
				cardSeed("sensei-hon-yomimasu", "先生は本を読みます", "せんせいはほんをよみます", "sensei wa hon o yomimasu", "The teacher reads a book.", content.CardTypePhrase, "先生は = teacher as topic; 本を = book as object; 読みます = read"),
				cardSeed("gakusei-mizu-nomimasu", "学生は水を飲みます", "がくせいはみずをのみます", "gakusei wa mizu o nomimasu", "The student drinks water.", content.CardTypePhrase, "学生は = student as topic; 水を = water as object; 飲みます = drink"),
				cardSeed("tomodachi-eiga-mimasu", "友達と映画を見ます", "ともだちとえいがをみます", "tomodachi to eiga o mimasu", "I watch a movie with a friend.", content.CardTypePhrase, "友達と = with a friend; 映画を = movie as object; 見ます = watch"),
			},
		},
	}...)
	return append(lessons, beginner200LessonSeeds()...)
}

const beginner200StartLevelID = "lesson-hi-n5-sentence-trees"

var beginner200KanjiRows = []string{
	"一二三四五六七八九十",
	"百千万円年月日時分半",
	"週今毎何先生学校小中",
	"大上下左右前後内外北",
	"南東西人男女子母父友",
	"名国語本書読聞話見行",
	"来帰食飲買出入休会社",
	"店駅車電山川田天気雨",
	"雪火水木金土花魚犬白",
	"黒赤青安高新古長多少",
	"早午間私主住所町村市",
	"都道京海空家室部屋門",
	"開閉立座歩走止動働作",
	"使持待言思知勉強教習",
	"考答問題文字英漢料理",
	"茶肉牛鳥米野菜果物朝",
	"昼夜晩春夏秋冬明暗近",
	"遠広狭弱好悪正元有無",
	"事方同別次最初終色音",
	"楽歌写真旅病院薬医者",
}

// Beginner200KanjiRows returns the visible N5 kanji trail in lesson order.
func Beginner200KanjiRows() []string {
	return append([]string(nil), beginner200KanjiRows...)
}

type beginnerKanjiSeed struct {
	Char        string
	Kana        string
	Romaji      string
	Meaning     string
	Word        string
	WordKana    string
	WordRomaji  string
	WordMeaning string
}

type beginnerSentenceSeed struct {
	Text    string
	Kana    string
	Romaji  string
	Meaning string
	Tree    []treeNoteSeed
}

type treeNoteSeed struct {
	Label string
	Body  string
}

type beginnerKanjiGroup struct {
	Number    int
	Title     string
	Kanji     []beginnerKanjiSeed
	Sentences []beginnerSentenceSeed
}

func beginner200LessonSeeds() []lessonSeed {
	groups := beginner200Groups()
	ankiBoosts := ankiN5VerbBoostLessons()
	ankiBoostIndex := 0
	ankiBoostAfterGroup := map[int]bool{3: true, 6: true, 9: true, 12: true, 15: true, 18: true, 20: true}
	lessons := make([]lessonSeed, 0, len(groups)*2+len(ankiBoosts))
	requiredLessonID := beginner200StartLevelID
	for _, group := range groups {
		coreID := fmt.Sprintf("lesson-b200-g%02d-core", group.Number)
		sentenceID := fmt.Sprintf("lesson-b200-g%02d-sentences", group.Number)
		requiredPoints := 6000 + (group.Number-1)*700
		lessons = append(lessons, lessonSeed{
			ID:               coreID,
			Title:            fmt.Sprintf("Beginner 200.%02d - %s Core", group.Number, group.Title),
			Description:      fmt.Sprintf("Light up kanji %03d-%03d with readings and usable words.", (group.Number-1)*10+1, group.Number*10),
			DocumentTitle:    "SQLite Lesson - 日 Foundation",
			RequiredLessonID: requiredLessonID,
			RequiredPoints:   requiredPoints,
			Cards:            beginnerCoreCards(group),
		})
		lessons = append(lessons, lessonSeed{
			ID:               sentenceID,
			Title:            fmt.Sprintf("Beginner 200.%02d - %s Sentence Gate", group.Number, group.Title),
			Description:      "Use the new kanji inside short sentence trees before the next beach opens.",
			DocumentTitle:    "SQLite Lesson - 日 Foundation",
			RequiredLessonID: coreID,
			RequiredPoints:   requiredPoints,
			Cards:            beginnerSentenceCards(group),
		})
		requiredLessonID = sentenceID
		if ankiBoostAfterGroup[group.Number] && ankiBoostIndex < len(ankiBoosts) {
			boost := ankiBoosts[ankiBoostIndex]
			boost.RequiredLessonID = sentenceID
			boost.RequiredPoints = requiredPoints
			lessons = append(lessons, boost)
			requiredLessonID = boost.ID
			ankiBoostIndex++
		}
	}
	return lessons
}

func beginnerCoreCards(group beginnerKanjiGroup) []lessonCardSeed {
	cards := make([]lessonCardSeed, 0, len(group.Kanji)*2)
	for i, seed := range group.Kanji {
		index := (group.Number-1)*10 + i + 1
		cardTags := fmt.Sprintf("sqlite|日-foundation|b200|b200-g%02d|b200-index-%03d", group.Number, index)
		cards = append(cards, lessonCardSeed{
			ID:         fmt.Sprintf("lesson-b200-%03d-kanji", index),
			Text:       seed.Char,
			Kanji:      seed.Char,
			Kana:       seed.Kana,
			RomajiHint: seed.Romaji,
			Meaning:    seed.Meaning,
			Type:       content.CardTypeKanjiReading,
			Notes:      fmt.Sprintf("grid %03d|reading %s = %s", index, seed.Char, seed.Kana),
			Tags:       cardTags + "|b200-kanji",
		})
		cards = append(cards, lessonCardSeed{
			ID:         fmt.Sprintf("lesson-b200-%03d-word", index),
			Text:       seed.Word,
			Kanji:      seed.Word,
			Kana:       seed.WordKana,
			RomajiHint: seed.WordRomaji,
			Meaning:    seed.WordMeaning,
			Type:       content.CardTypeWord,
			Notes:      fmt.Sprintf("uses %s / %s", seed.Char, seed.Kana),
			Tags:       cardTags + "|b200-word",
		})
	}
	return cards
}

func beginnerSentenceCards(group beginnerKanjiGroup) []lessonCardSeed {
	cards := make([]lessonCardSeed, 0, len(group.Sentences))
	for i, seed := range group.Sentences {
		cards = append(cards, lessonCardSeed{
			ID:         fmt.Sprintf("lesson-b200-g%02d-s%02d", group.Number, i+1),
			Text:       seed.Text,
			Kanji:      seed.Text,
			Kana:       seed.Kana,
			RomajiHint: seed.Romaji,
			Meaning:    seed.Meaning,
			Type:       content.CardTypePhrase,
			Notes:      treeNotes(seed.Tree),
			Tags:       fmt.Sprintf("sqlite|日-foundation|b200|b200-g%02d|b200-sentence", group.Number),
		})
	}
	return cards
}

func treeNotes(notes []treeNoteSeed) string {
	parts := make([]string, 0, len(notes))
	for _, note := range notes {
		parts = append(parts, fmt.Sprintf("tree %s: %s", note.Label, note.Body))
	}
	return strings.Join(parts, "|")
}

func beginner200Groups() []beginnerKanjiGroup {
	groups := []beginnerKanjiGroup{
		{
			Number: 1,
			Title:  "Numbers And Time",
			Kanji: []beginnerKanjiSeed{
				{Char: "一", Kana: "いち", Romaji: "ichi", Meaning: "one", Word: "一つ", WordKana: "ひとつ", WordRomaji: "hitotsu", WordMeaning: "one thing"},
				{Char: "二", Kana: "に", Romaji: "ni", Meaning: "two", Word: "二つ", WordKana: "ふたつ", WordRomaji: "futatsu", WordMeaning: "two things"},
				{Char: "三", Kana: "さん", Romaji: "san", Meaning: "three", Word: "三日", WordKana: "みっか", WordRomaji: "mikka", WordMeaning: "third day"},
				{Char: "四", Kana: "よん", Romaji: "yon", Meaning: "four", Word: "四日", WordKana: "よっか", WordRomaji: "yokka", WordMeaning: "fourth day"},
				{Char: "五", Kana: "ご", Romaji: "go", Meaning: "five", Word: "五日", WordKana: "いつか", WordRomaji: "itsuka", WordMeaning: "fifth day"},
				{Char: "六", Kana: "ろく", Romaji: "roku", Meaning: "six", Word: "六日", WordKana: "むいか", WordRomaji: "muika", WordMeaning: "sixth day"},
				{Char: "七", Kana: "なな", Romaji: "nana", Meaning: "seven", Word: "七日", WordKana: "なのか", WordRomaji: "nanoka", WordMeaning: "seventh day"},
				{Char: "八", Kana: "はち", Romaji: "hachi", Meaning: "eight", Word: "八日", WordKana: "ようか", WordRomaji: "youka", WordMeaning: "eighth day"},
				{Char: "九", Kana: "きゅう", Romaji: "kyuu", Meaning: "nine", Word: "九日", WordKana: "ここのか", WordRomaji: "kokonoka", WordMeaning: "ninth day"},
				{Char: "十", Kana: "じゅう", Romaji: "juu", Meaning: "ten", Word: "十日", WordKana: "とおか", WordRomaji: "tooka", WordMeaning: "tenth day"},
			},
			Sentences: []beginnerSentenceSeed{
				{Text: "一月一日は休みです", Kana: "いちがつついたちはやすみです", Romaji: "ichi gatsu tsuitachi wa yasumi desu", Meaning: "January first is a day off.", Tree: []treeNoteSeed{{"time", "一月一日 / いちがつついたち = January first"}, {"topic", "は = marks the day as the frame"}, {"state", "休みです / やすみです = is a rest day"}}},
				{Text: "十日で十まで数えます", Kana: "とおかでじゅうまでかぞえます", Romaji: "tooka de juu made kazoemasu", Meaning: "In ten days, I count to ten.", Tree: []treeNoteSeed{{"time", "十日で / とおかで = in ten days"}, {"limit", "十まで / じゅうまで = up to ten"}, {"verb", "数えます / かぞえます = count politely"}}},
				{Text: "五円と十円を見ます", Kana: "ごえんとじゅうえんをみます", Romaji: "go en to juu en o mimasu", Meaning: "I look at five yen and ten yen.", Tree: []treeNoteSeed{{"object", "五円と十円 / ごえんとじゅうえん = five yen and ten yen"}, {"particle", "を = marks the thing seen"}, {"verb", "見ます / みます = look at"}}},
				{Text: "三日と四日に学校へ行きます", Kana: "みっかとよっかにがっこうへいきます", Romaji: "mikka to yokka ni gakkou e ikimasu", Meaning: "I go to school on the third and fourth.", Tree: []treeNoteSeed{{"time", "三日と四日に / みっかとよっかに = on the third and fourth"}, {"place", "学校へ / がっこうへ = toward school"}, {"verb", "行きます / いきます = go"}}},
				{Text: "七日から八日までいます", Kana: "なのかからようかまでいます", Romaji: "nanoka kara youka made imasu", Meaning: "I am there from the seventh to the eighth.", Tree: []treeNoteSeed{{"start", "七日から / なのかから = from the seventh"}, {"end", "八日まで / ようかまで = until the eighth"}, {"verb", "います = am there"}}},
			},
		},
		{
			Number: 2,
			Title:  "People And School",
			Kanji: []beginnerKanjiSeed{
				{Char: "百", Kana: "ひゃく", Romaji: "hyaku", Meaning: "hundred", Word: "百円", WordKana: "ひゃくえん", WordRomaji: "hyaku en", WordMeaning: "100 yen"},
				{Char: "千", Kana: "せん", Romaji: "sen", Meaning: "thousand", Word: "千円", WordKana: "せんえん", WordRomaji: "sen en", WordMeaning: "1000 yen"},
				{Char: "万", Kana: "まん", Romaji: "man", Meaning: "ten thousand", Word: "一万円", WordKana: "いちまんえん", WordRomaji: "ichi man en", WordMeaning: "10000 yen"},
				{Char: "円", Kana: "えん", Romaji: "en", Meaning: "yen; circle", Word: "百円", WordKana: "ひゃくえん", WordRomaji: "hyaku en", WordMeaning: "100 yen"},
				{Char: "年", Kana: "ねん", Romaji: "nen", Meaning: "year", Word: "今年", WordKana: "ことし", WordRomaji: "kotoshi", WordMeaning: "this year"},
				{Char: "月", Kana: "がつ", Romaji: "gatsu", Meaning: "month; moon", Word: "一月", WordKana: "いちがつ", WordRomaji: "ichi gatsu", WordMeaning: "January"},
				{Char: "日", Kana: "にち", Romaji: "nichi", Meaning: "day; sun", Word: "日曜日", WordKana: "にちようび", WordRomaji: "nichiyoubi", WordMeaning: "Sunday"},
				{Char: "時", Kana: "じ", Romaji: "ji", Meaning: "hour; time", Word: "一時", WordKana: "いちじ", WordRomaji: "ichi ji", WordMeaning: "one o'clock"},
				{Char: "分", Kana: "ふん", Romaji: "fun", Meaning: "minute; part", Word: "五分", WordKana: "ごふん", WordRomaji: "go fun", WordMeaning: "five minutes"},
				{Char: "半", Kana: "はん", Romaji: "han", Meaning: "half", Word: "半分", WordKana: "はんぶん", WordRomaji: "hanbun", WordMeaning: "half"},
			},
			Sentences: []beginnerSentenceSeed{
				{Text: "今年は一万円を使います", Kana: "ことしはいちまんえんをつかいます", Romaji: "kotoshi wa ichi man en o tsukaimasu", Meaning: "This year I use ten thousand yen.", Tree: []treeNoteSeed{{"time", "今年は / ことしは = this year as topic"}, {"object", "一万円を / いちまんえんを = 10000 yen as object"}, {"verb", "使います / つかいます = use"}}},
				{Text: "百円と千円があります", Kana: "ひゃくえんとせんえんがあります", Romaji: "hyaku en to sen en ga arimasu", Meaning: "There are 100 yen and 1000 yen.", Tree: []treeNoteSeed{{"thing", "百円と千円 / ひゃくえんとせんえん = 100 yen and 1000 yen"}, {"subject", "が = marks what exists"}, {"verb", "あります = exists"}}},
				{Text: "一月は日曜日が多いです", Kana: "いちがつはにちようびがおおいです", Romaji: "ichi gatsu wa nichiyoubi ga ooi desu", Meaning: "January has many Sundays.", Tree: []treeNoteSeed{{"time", "一月は / いちがつは = January as topic"}, {"subject", "日曜日が / にちようびが = Sundays"}, {"state", "多いです / おおいです = are many"}}},
				{Text: "一時半に来ます", Kana: "いちじはんにきます", Romaji: "ichi ji han ni kimasu", Meaning: "I come at one thirty.", Tree: []treeNoteSeed{{"time", "一時半に / いちじはんに = at one thirty"}, {"verb", "来ます / きます = come"}, {"shape", "time + に anchors the action"}}},
				{Text: "五分で本を読みます", Kana: "ごふんでほんをよみます", Romaji: "go fun de hon o yomimasu", Meaning: "I read a book in five minutes.", Tree: []treeNoteSeed{{"time", "五分で / ごふんで = in five minutes"}, {"object", "本を / ほんを = book as object"}, {"verb", "読みます / よみます = read"}}},
			},
		},
		{
			Number: 3,
			Title:  "Weeks And School",
			Kanji: []beginnerKanjiSeed{
				{Char: "週", Kana: "しゅう", Romaji: "shuu", Meaning: "week", Word: "今週", WordKana: "こんしゅう", WordRomaji: "konshuu", WordMeaning: "this week"},
				{Char: "今", Kana: "いま", Romaji: "ima", Meaning: "now", Word: "今日", WordKana: "きょう", WordRomaji: "kyou", WordMeaning: "today"},
				{Char: "毎", Kana: "まい", Romaji: "mai", Meaning: "every", Word: "毎日", WordKana: "まいにち", WordRomaji: "mainichi", WordMeaning: "every day"},
				{Char: "何", Kana: "なに", Romaji: "nani", Meaning: "what", Word: "何時", WordKana: "なんじ", WordRomaji: "nanji", WordMeaning: "what time"},
				{Char: "先", Kana: "せん", Romaji: "sen", Meaning: "ahead; previous", Word: "先生", WordKana: "せんせい", WordRomaji: "sensei", WordMeaning: "teacher"},
				{Char: "生", Kana: "せい", Romaji: "sei", Meaning: "life; student", Word: "学生", WordKana: "がくせい", WordRomaji: "gakusei", WordMeaning: "student"},
				{Char: "学", Kana: "がく", Romaji: "gaku", Meaning: "study", Word: "学校", WordKana: "がっこう", WordRomaji: "gakkou", WordMeaning: "school"},
				{Char: "校", Kana: "こう", Romaji: "kou", Meaning: "school", Word: "学校", WordKana: "がっこう", WordRomaji: "gakkou", WordMeaning: "school"},
				{Char: "小", Kana: "しょう", Romaji: "shou", Meaning: "small", Word: "小さい", WordKana: "ちいさい", WordRomaji: "chiisai", WordMeaning: "small"},
				{Char: "中", Kana: "なか", Romaji: "naka", Meaning: "inside; middle", Word: "中学校", WordKana: "ちゅうがっこう", WordRomaji: "chuugakkou", WordMeaning: "junior high school"},
			},
			Sentences: []beginnerSentenceSeed{
				{Text: "今週は毎日学校へ行きます", Kana: "こんしゅうはまいにちがっこうへいきます", Romaji: "konshuu wa mainichi gakkou e ikimasu", Meaning: "This week I go to school every day.", Tree: []treeNoteSeed{{"time", "今週は / こんしゅうは = this week as topic"}, {"rhythm", "毎日 / まいにち = every day"}, {"place", "学校へ / がっこうへ = toward school"}, {"verb", "行きます / いきます = go"}}},
				{Text: "先生は何時に来ますか", Kana: "せんせいはなんじにきますか", Romaji: "sensei wa nanji ni kimasu ka", Meaning: "What time does the teacher come?", Tree: []treeNoteSeed{{"topic", "先生は / せんせいは = teacher as topic"}, {"time", "何時に / なんじに = at what time"}, {"verb", "来ますか / きますか = comes?"}}},
				{Text: "学生は中学校で勉強します", Kana: "がくせいはちゅうがっこうでべんきょうします", Romaji: "gakusei wa chuugakkou de benkyou shimasu", Meaning: "The student studies at junior high school.", Tree: []treeNoteSeed{{"topic", "学生は / がくせいは = student as topic"}, {"place", "中学校で / ちゅうがっこうで = at junior high"}, {"verb", "勉強します / べんきょうします = study"}}},
				{Text: "今年の一月に先生に会います", Kana: "ことしのいちがつにせんせいにあいます", Romaji: "kotoshi no ichi gatsu ni sensei ni aimasu", Meaning: "I meet the teacher in January this year.", Tree: []treeNoteSeed{{"time", "今年の一月に / ことしのいちがつに = in January this year"}, {"target", "先生に / せんせいに = to the teacher"}, {"verb", "会います / あいます = meet"}}},
				{Text: "小さい学校で本を読みます", Kana: "ちいさいがっこうでほんをよみます", Romaji: "chiisai gakkou de hon o yomimasu", Meaning: "I read a book at a small school.", Tree: []treeNoteSeed{{"place", "小さい学校で / ちいさいがっこうで = at a small school"}, {"object", "本を / ほんを = book as object"}, {"verb", "読みます / よみます = read"}}},
			},
		},
		{
			Number: 4,
			Title:  "Size And Direction",
			Kanji: []beginnerKanjiSeed{
				{Char: "大", Kana: "おお", Romaji: "oo", Meaning: "big", Word: "大きい", WordKana: "おおきい", WordRomaji: "ookii", WordMeaning: "big"},
				{Char: "上", Kana: "うえ", Romaji: "ue", Meaning: "up; above", Word: "上手", WordKana: "じょうず", WordRomaji: "jouzu", WordMeaning: "skillful"},
				{Char: "下", Kana: "した", Romaji: "shita", Meaning: "down; below", Word: "下手", WordKana: "へた", WordRomaji: "heta", WordMeaning: "unskillful"},
				{Char: "左", Kana: "ひだり", Romaji: "hidari", Meaning: "left", Word: "左手", WordKana: "ひだりて", WordRomaji: "hidarite", WordMeaning: "left hand"},
				{Char: "右", Kana: "みぎ", Romaji: "migi", Meaning: "right", Word: "右手", WordKana: "みぎて", WordRomaji: "migite", WordMeaning: "right hand"},
				{Char: "前", Kana: "まえ", Romaji: "mae", Meaning: "front; before", Word: "午前", WordKana: "ごぜん", WordRomaji: "gozen", WordMeaning: "morning"},
				{Char: "後", Kana: "あと", Romaji: "ato", Meaning: "after; behind", Word: "午後", WordKana: "ごご", WordRomaji: "gogo", WordMeaning: "afternoon"},
				{Char: "内", Kana: "うち", Romaji: "uchi", Meaning: "inside", Word: "内側", WordKana: "うちがわ", WordRomaji: "uchigawa", WordMeaning: "inside"},
				{Char: "外", Kana: "そと", Romaji: "soto", Meaning: "outside", Word: "外国", WordKana: "がいこく", WordRomaji: "gaikoku", WordMeaning: "foreign country"},
				{Char: "北", Kana: "きた", Romaji: "kita", Meaning: "north", Word: "北口", WordKana: "きたぐち", WordRomaji: "kitaguchi", WordMeaning: "north exit"},
			},
			Sentences: []beginnerSentenceSeed{
				{Text: "大きい本を上におきます", Kana: "おおきいほんをうえにおきます", Romaji: "ookii hon o ue ni okimasu", Meaning: "I put the big book above.", Tree: []treeNoteSeed{{"object", "大きい本を / おおきいほんを = the big book"}, {"place", "上に / うえに = above"}, {"verb", "おきます = put"}}},
				{Text: "左手と右手を見ます", Kana: "ひだりてとみぎてをみます", Romaji: "hidarite to migite o mimasu", Meaning: "I look at my left and right hands.", Tree: []treeNoteSeed{{"object", "左手と右手を / ひだりてとみぎてを = left and right hands"}, {"particle", "を = marks what is seen"}, {"verb", "見ます / みます = look at"}}},
				{Text: "午前に行き午後に帰ります", Kana: "ごぜんにいきごごにかえります", Romaji: "gozen ni iki gogo ni kaerimasu", Meaning: "I go in the morning and return in the afternoon.", Tree: []treeNoteSeed{{"time", "午前に / ごぜんに = in the morning"}, {"time", "午後に / ごごに = in the afternoon"}, {"verbs", "行き / いき + 帰ります / かえります = go and return"}}},
				{Text: "つくえの下に本があります", Kana: "つくえのしたにほんがあります", Romaji: "tsukue no shita ni hon ga arimasu", Meaning: "There is a book under the desk.", Tree: []treeNoteSeed{{"place", "下に / したに = under"}, {"thing", "本が / ほんが = book as subject"}, {"verb", "あります = exists"}}},
				{Text: "前と後を見ます", Kana: "まえとあとをみます", Romaji: "mae to ato o mimasu", Meaning: "I look front and back.", Tree: []treeNoteSeed{{"object", "前と後を / まえとあとを = front and back"}, {"verb", "見ます / みます = look"}, {"shape", "AとB = A and B"}}},
			},
		},
		{
			Number: 5,
			Title:  "Directions And People",
			Kanji: []beginnerKanjiSeed{
				{Char: "南", Kana: "みなみ", Romaji: "minami", Meaning: "south", Word: "南口", WordKana: "みなみぐち", WordRomaji: "minamiguchi", WordMeaning: "south exit"},
				{Char: "東", Kana: "ひがし", Romaji: "higashi", Meaning: "east", Word: "東京", WordKana: "とうきょう", WordRomaji: "toukyou", WordMeaning: "Tokyo"},
				{Char: "西", Kana: "にし", Romaji: "nishi", Meaning: "west", Word: "西口", WordKana: "にしぐち", WordRomaji: "nishiguchi", WordMeaning: "west exit"},
				{Char: "人", Kana: "ひと", Romaji: "hito", Meaning: "person", Word: "日本人", WordKana: "にほんじん", WordRomaji: "nihonjin", WordMeaning: "Japanese person"},
				{Char: "男", Kana: "おとこ", Romaji: "otoko", Meaning: "man", Word: "男の子", WordKana: "おとこのこ", WordRomaji: "otoko no ko", WordMeaning: "boy"},
				{Char: "女", Kana: "おんな", Romaji: "onna", Meaning: "woman", Word: "女の子", WordKana: "おんなのこ", WordRomaji: "onna no ko", WordMeaning: "girl"},
				{Char: "子", Kana: "こ", Romaji: "ko", Meaning: "child", Word: "子ども", WordKana: "こども", WordRomaji: "kodomo", WordMeaning: "child"},
				{Char: "母", Kana: "はは", Romaji: "haha", Meaning: "mother", Word: "お母さん", WordKana: "おかあさん", WordRomaji: "okaasan", WordMeaning: "mother"},
				{Char: "父", Kana: "ちち", Romaji: "chichi", Meaning: "father", Word: "お父さん", WordKana: "おとうさん", WordRomaji: "otousan", WordMeaning: "father"},
				{Char: "友", Kana: "とも", Romaji: "tomo", Meaning: "friend", Word: "友達", WordKana: "ともだち", WordRomaji: "tomodachi", WordMeaning: "friend"},
			},
			Sentences: []beginnerSentenceSeed{
				{Text: "南口から東へ行きます", Kana: "みなみぐちからひがしへいきます", Romaji: "minamiguchi kara higashi e ikimasu", Meaning: "I go east from the south exit.", Tree: []treeNoteSeed{{"start", "南口から / みなみぐちから = from the south exit"}, {"place", "東へ / ひがしへ = toward east"}, {"verb", "行きます / いきます = go"}}},
				{Text: "西口で友達に会います", Kana: "にしぐちでともだちにあいます", Romaji: "nishiguchi de tomodachi ni aimasu", Meaning: "I meet a friend at the west exit.", Tree: []treeNoteSeed{{"place", "西口で / にしぐちで = at the west exit"}, {"target", "友達に / ともだちに = to a friend"}, {"verb", "会います / あいます = meet"}}},
				{Text: "日本人の友達が来ます", Kana: "にほんじんのともだちがきます", Romaji: "nihonjin no tomodachi ga kimasu", Meaning: "A Japanese friend comes.", Tree: []treeNoteSeed{{"person", "日本人の友達 / にほんじんのともだち = Japanese friend"}, {"subject", "が = marks who comes"}, {"verb", "来ます / きます = come"}}},
				{Text: "男の子と女の子は本を読みます", Kana: "おとこのことおんなのこはほんをよみます", Romaji: "otoko no ko to onna no ko wa hon o yomimasu", Meaning: "The boy and girl read a book.", Tree: []treeNoteSeed{{"topic", "男の子と女の子は = boy and girl as topic"}, {"object", "本を / ほんを = book as object"}, {"verb", "読みます / よみます = read"}}},
				{Text: "お母さんとお父さんに会います", Kana: "おかあさんとおとうさんにあいます", Romaji: "okaasan to otousan ni aimasu", Meaning: "I meet mother and father.", Tree: []treeNoteSeed{{"target", "お母さんとお父さんに = to mother and father"}, {"verb", "会います / あいます = meet"}, {"shape", "AとB = A and B"}}},
			},
		},
	}
	return append(groups, laterBeginner200Groups()...)
}

func cardSeed(id, text, kana, romaji, meaning string, cardType content.CardType, notes string) lessonCardSeed {
	return lessonCardSeed{
		ID:         "lesson-hi-" + id,
		Text:       text,
		Kanji:      text,
		Kana:       kana,
		RomajiHint: romaji,
		Meaning:    meaning,
		Type:       cardType,
		Notes:      notes,
		Tags:       "sqlite|日-foundation",
	}
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
