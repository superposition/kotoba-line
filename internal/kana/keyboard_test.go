package kana

import "testing"

func TestFromKeyboard(t *testing.T) {
	for input, want := range map[string]string{
		"nihon":          "にほん",
		"nippon":         "にっぽん",
		"mainichi":       "まいにち",
		"kokkai":         "こっかい",
		"shouchou":       "しょうちょう",
		"tennou":         "てんのう",
		"dai-ichi-jou":   "だいいちじょう",
		"son suru":       "そんする",
		"hi ga kureru":   "ひがくれる",
		"hon wo":         "ほんを",
		"seitou ni":      "せいとうに",
		"juuichi-nichi":  "じゅういちにち",
		"kasugachou":     "かすがちょう",
		"nihonkokuminwa": "にほんこくみんわ",
	} {
		got, ok := FromKeyboard(input)
		if !ok {
			t.Fatalf("FromKeyboard(%q) did not parse", input)
		}
		if got != want {
			t.Fatalf("FromKeyboard(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestFromKeyboardForTargetUsesTargetScript(t *testing.T) {
	for _, tt := range []struct {
		target string
		input  string
		want   string
	}{
		{target: "か", input: "ka", want: "か"},
		{target: "カ", input: "ka", want: "カ"},
		{target: "ニホン", input: "nihon", want: "ニホン"},
	} {
		got, ok := FromKeyboardForTarget(tt.target, tt.input)
		if !ok {
			t.Fatalf("FromKeyboardForTarget(%q, %q) did not parse", tt.target, tt.input)
		}
		if got != tt.want {
			t.Fatalf("FromKeyboardForTarget(%q, %q) = %q, want %q", tt.target, tt.input, got, tt.want)
		}
	}
}

func TestMatchesAnswer(t *testing.T) {
	if !MatchesAnswer("にっぽん", "nippon", "nippon") {
		t.Fatalf("nippon should match にっぽん through keyboard input")
	}
	if !MatchesAnswer("カ", "ka", "ka") {
		t.Fatalf("keyboard input should match katakana target")
	}
	if MatchesAnswer("カ", "ka", "か") {
		t.Fatalf("wrong-script exact kana should not match katakana target")
	}
	if !MatchesAnswer("にほんこくみんは", "nihon-kokumin wa", "nihon-kokumin-wa") {
		t.Fatalf("particle は phrase should match through romaji hint alias")
	}
	if !MatchesAnswer("もとづく", "motozuku", "motozuku") {
		t.Fatalf("historical づ reading should match through hint alias")
	}
	if MatchesAnswer("にち", "nichi", "nihon") {
		t.Fatalf("wrong keyboard input should not match")
	}
}

func TestAnswerInputState(t *testing.T) {
	target := "ほんじつはまことにありがとうございました"
	romaji := "honjitsu wa makoto ni arigatou gozaimashita"

	for input, want := range map[string]AnswerInputStatus{
		"":              AnswerInputEmpty,
		"ほんじつ":          AnswerInputPossible,
		"honjitsuha":    AnswerInputPossible,
		"honjitsu ha m": AnswerInputPossible,
		"honjitsuwa":    AnswerInputWrong,
		"honjitsu wa m": AnswerInputWrong,
		"honjitsuwaar":  AnswerInputWrong,
		"honjitsu ha makoto ni arigatou gozaimashita": AnswerInputCorrect,
		"honjitsu wa makoto ni arigatou gozaimashita": AnswerInputCorrect,
	} {
		if got := AnswerInputState(target, romaji, input); got != want {
			t.Fatalf("AnswerInputState(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestAnswerInputStateAllowsSpaceKeyProgress(t *testing.T) {
	if got := AnswerInputState("ひがくれる", "hi ga kureru", "hi "); got != AnswerInputPossible {
		t.Fatalf("AnswerInputState(hi space) = %q, want possible", got)
	}
	if got := AnswerInputState("ひがくれる", "hi ga kureru", "hi ga kureru"); got != AnswerInputCorrect {
		t.Fatalf("AnswerInputState(full spaced romaji) = %q, want correct", got)
	}
}

func TestTypingSequence(t *testing.T) {
	for _, tt := range []struct {
		target string
		hint   string
		want   string
	}{
		{target: "ひがくれる", hint: "hi ga kureru", want: "hi ga kureru"},
		{target: "だいいちじょう", hint: "dai-ichi-jou", want: "dai ichi jou"},
		{target: "もとづく", hint: "motozuku / motoduku", want: "motoduku"},
		{target: "せんせいはほんをよみます", hint: "sensei wa hon o yomimasu", want: "sensei ha hon wo yomimasu"},
		{target: "ほんを", hint: "hon o", want: "hon wo"},
		{target: "わたしは", hint: "watashi wa", want: "watashi ha"},
		{target: "きょうはがっこうへいきます", hint: "kyou wa gakkou e ikimasu", want: "kyou ha gakkou he ikimasu"},
		{target: "ひ", hint: "", want: "hi"},
		{target: "カタカナ", hint: "katakana", want: "katakana"},
	} {
		if got := TypingSequence(tt.target, tt.hint); got != tt.want {
			t.Fatalf("TypingSequence(%q, %q) = %q, want %q", tt.target, tt.hint, got, tt.want)
		}
	}
}

func TestPreview(t *testing.T) {
	if got := Preview("nippon"); got != "にっぽん" {
		t.Fatalf("Preview(nippon) = %q, want にっぽん", got)
	}
	if got := Preview("にほん"); got != "" {
		t.Fatalf("Preview(kana) = %q, want empty", got)
	}
	if got := PreviewForTarget("カ", "ka"); got != "カ" {
		t.Fatalf("PreviewForTarget(katakana ka) = %q, want カ", got)
	}
}

func TestBasicRowsExposeHiraganaKatakanaComparison(t *testing.T) {
	rows := BasicRows()
	if len(rows) != 11 {
		t.Fatalf("row count = %d, want 11", len(rows))
	}
	if rows[1].Label != "k" || rows[1].Cells[0].Romaji != "ka" || rows[1].Cells[0].Hiragana != "か" || rows[1].Cells[0].Katakana != "カ" {
		t.Fatalf("bad k-row first cell: %#v", rows[1])
	}
	last := rows[len(rows)-1]
	if last.Cells[0].Hiragana != "ん" || last.Cells[0].Katakana != "ン" {
		t.Fatalf("bad terminal n row: %#v", last)
	}
}
