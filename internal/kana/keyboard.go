package kana

import (
	"strings"
	"unicode"
)

type AnswerInputStatus string

const (
	AnswerInputEmpty    AnswerInputStatus = "empty"
	AnswerInputPossible AnswerInputStatus = "possible"
	AnswerInputCorrect  AnswerInputStatus = "correct"
	AnswerInputWrong    AnswerInputStatus = "wrong"
)

type Script string

const (
	ScriptHiragana Script = "hiragana"
	ScriptKatakana Script = "katakana"
)

type Cell struct {
	Row      string
	Vowel    string
	Romaji   string
	Hiragana string
	Katakana string
}

type Row struct {
	Label string
	Cells []Cell
}

// MatchesAnswer accepts either exact kana or plain keyboard syllables. The
// target stays Japanese, but players do not need to fight a system IME popup.
func MatchesAnswer(targetKana, romajiHint, input string) bool {
	answer := strings.TrimSpace(input)
	if answer == "" {
		return false
	}
	if answer == targetKana {
		return true
	}

	normalized := normalizeKeyboard(answer)
	for _, hint := range hintAliases(romajiHint) {
		if normalized == normalizeKeyboard(hint) {
			return true
		}
	}

	converted, ok := FromKeyboardForTarget(targetKana, answer)
	return ok && converted == targetKana
}

func TypingSequence(targetKana, romajiHint string) string {
	target := strings.TrimSpace(targetKana)
	if target != "" {
		for _, hint := range hintAliases(romajiHint) {
			if sequence, ok := kanaAlignedTypingSequence(target, hint); ok {
				return sequence
			}
		}
		if sequence, ok := kanaTypingSequence(target); ok {
			return sequence
		}
	}

	for _, hint := range hintAliases(romajiHint) {
		if sequence := normalizeTypingSequence(hint); sequence != "" {
			return sequence
		}
	}
	return target
}

// AnswerInputState reports whether the current input can still become the
// target answer. It is intentionally prefix-oriented so the UI can flag a wrong
// turn while the player is still typing instead of waiting for Enter.
func AnswerInputState(targetKana, romajiHint, input string) AnswerInputStatus {
	answer := strings.TrimSpace(input)
	if answer == "" {
		return AnswerInputEmpty
	}
	if MatchesAnswer(targetKana, romajiHint, answer) {
		return AnswerInputCorrect
	}
	if strings.HasPrefix(targetKana, answer) {
		return AnswerInputPossible
	}

	sequence := TypingSequence(targetKana, romajiHint)
	if typed := normalizeTypingSequence(answer); sequence != "" && typed != "" {
		if strings.HasPrefix(sequence, typed) {
			return AnswerInputPossible
		}
	}

	normalized := normalizeKeyboard(answer)
	if normalized == "" {
		return AnswerInputWrong
	}
	if keyboardPrefixCanMatch(targetKana, normalized) {
		return AnswerInputPossible
	}
	return AnswerInputWrong
}

func kanaAlignedTypingSequence(targetKana, hint string) (string, bool) {
	sequence := normalizeTypingSequence(hint)
	if sequence == "" {
		return "", false
	}

	remaining := targetKana
	tokens := strings.Fields(sequence)
	out := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if converted, ok := FromKeyboardForTarget(remaining, token); ok && strings.HasPrefix(remaining, converted) {
			out = append(out, token)
			remaining = remaining[len(converted):]
			continue
		}
		if replacement, advance, ok := literalParticleToken(remaining, token); ok {
			out = append(out, replacement)
			remaining = remaining[advance:]
			continue
		}
		return "", false
	}
	if remaining != "" {
		return "", false
	}
	return strings.Join(out, " "), true
}

func literalParticleToken(remaining, token string) (string, int, bool) {
	switch token {
	case "wa":
		if strings.HasPrefix(remaining, "は") {
			return "ha", len("は"), true
		}
	case "o":
		if strings.HasPrefix(remaining, "を") {
			return "wo", len("を"), true
		}
	case "e":
		if strings.HasPrefix(remaining, "へ") {
			return "he", len("へ"), true
		}
	}
	return "", 0, false
}

func kanaTypingSequence(targetKana string) (string, bool) {
	runes := []rune(strings.TrimSpace(targetKana))
	if len(runes) == 0 {
		return "", false
	}

	var out strings.Builder
	lastSpace := true
	for i := 0; i < len(runes); i++ {
		if unicode.IsSpace(runes[i]) {
			if !lastSpace {
				out.WriteRune(' ')
				lastSpace = true
			}
			continue
		}
		if runes[i] == 'っ' || runes[i] == 'ッ' {
			next, _, ok := kanaTokenFromRunes(runes, i+1)
			if !ok || next == "" || !isConsonant(next[0]) {
				return "", false
			}
			out.WriteByte(next[0])
			lastSpace = false
			continue
		}
		token, consumed, ok := kanaTokenFromRunes(runes, i)
		if !ok {
			return "", false
		}
		out.WriteString(token)
		lastSpace = false
		i += consumed - 1
	}
	return strings.TrimSpace(out.String()), true
}

func kanaTokenFromRunes(runes []rune, index int) (string, int, bool) {
	if index >= len(runes) {
		return "", 0, false
	}
	if index+1 < len(runes) {
		if token, ok := canonicalKanaKeyboard[string(runes[index:index+2])]; ok {
			return token, 2, true
		}
	}
	if token, ok := canonicalKanaKeyboard[string(runes[index])]; ok {
		return token, 1, true
	}
	return "", 0, false
}

// Preview returns the kana produced by plain keyboard syllables.
func Preview(input string) string {
	converted, ok := FromKeyboard(input)
	if !ok || converted == strings.TrimSpace(input) {
		return ""
	}
	return converted
}

// PreviewForTarget returns the kana produced by plain keyboard syllables in the
// target script. Exact kana input intentionally does not preview as a conversion.
func PreviewForTarget(targetKana, input string) string {
	converted, ok := FromKeyboardForTarget(targetKana, input)
	if !ok || converted == strings.TrimSpace(input) {
		return ""
	}
	return converted
}

// FromKeyboard converts simple Hepburn-style keyboard syllables to hiragana.
func FromKeyboard(input string) (string, bool) {
	s := normalizeKeyboard(input)
	if s == "" {
		return "", false
	}

	var out strings.Builder
	for i := 0; i < len(s); {
		if s[i] == 'n' {
			if i+1 == len(s) {
				out.WriteString("ん")
				i++
				continue
			}
			next := s[i+1]
			if next == 'n' || (!isVowel(next) && next != 'y') {
				out.WriteString("ん")
				i++
				continue
			}
		}

		if i+1 < len(s) && s[i] == s[i+1] && isConsonant(s[i]) && s[i] != 'n' {
			out.WriteString("っ")
			i++
			continue
		}

		matched := false
		for length := 3; length >= 1; length-- {
			if i+length > len(s) {
				continue
			}
			if kana, ok := keyboardKana[s[i:i+length]]; ok {
				out.WriteString(kana)
				i += length
				matched = true
				break
			}
		}
		if !matched {
			return "", false
		}
	}

	return out.String(), true
}

// FromKeyboardForTarget converts keyboard syllables to the script used by the
// target answer. Hiragana remains the default for mixed or non-kana targets.
func FromKeyboardForTarget(targetKana, input string) (string, bool) {
	converted, ok := FromKeyboard(input)
	if !ok {
		return "", false
	}
	if targetScript(targetKana) == ScriptKatakana {
		return ToKatakana(converted), true
	}
	return converted, true
}

func ToKatakana(input string) string {
	var out strings.Builder
	for _, r := range input {
		if r >= 'ぁ' && r <= 'ゖ' {
			out.WriteRune(r + 0x60)
			continue
		}
		out.WriteRune(r)
	}
	return out.String()
}

func ToHiragana(input string) string {
	var out strings.Builder
	for _, r := range input {
		if r >= 'ァ' && r <= 'ヶ' {
			out.WriteRune(r - 0x60)
			continue
		}
		out.WriteRune(r)
	}
	return out.String()
}

func targetScript(targetKana string) Script {
	hasKatakana := false
	hasHiragana := false
	for _, r := range targetKana {
		switch {
		case r >= 'ぁ' && r <= 'ゖ':
			hasHiragana = true
		case r >= 'ァ' && r <= 'ヶ':
			hasKatakana = true
		}
	}
	if hasKatakana && !hasHiragana {
		return ScriptKatakana
	}
	return ScriptHiragana
}

func keyboardPrefixCanMatch(targetKana, input string) bool {
	if targetKana == "" || input == "" {
		return false
	}
	return keyboardPrefixCanMatchFrom(ToHiragana(targetKana), input, "", map[string]bool{})
}

func keyboardPrefixCanMatchFrom(targetKana, remaining, out string, memo map[string]bool) bool {
	if !strings.HasPrefix(targetKana, out) {
		return false
	}
	if remaining == "" {
		return true
	}

	key := remaining + "\x00" + out
	if value, ok := memo[key]; ok {
		return value
	}
	memo[key] = false

	for token, kana := range keyboardKana {
		if strings.HasPrefix(token, remaining) && strings.HasPrefix(targetKana, out+kana) {
			memo[key] = true
			return true
		}
	}

	if remaining[0] == 'n' {
		if keyboardPrefixCanMatchFrom(targetKana, remaining[1:], out+"ん", memo) {
			memo[key] = true
			return true
		}
	}

	if len(remaining) >= 2 && remaining[0] == remaining[1] && isConsonant(remaining[0]) && remaining[0] != 'n' {
		if keyboardPrefixCanMatchFrom(targetKana, remaining[1:], out+"っ", memo) {
			memo[key] = true
			return true
		}
	}

	for length := 3; length >= 1; length-- {
		if length > len(remaining) {
			continue
		}
		if kana, ok := keyboardKana[remaining[:length]]; ok {
			if keyboardPrefixCanMatchFrom(targetKana, remaining[length:], out+kana, memo) {
				memo[key] = true
				return true
			}
		}
	}

	return false
}

func hintAliases(hint string) []string {
	fields := strings.FieldsFunc(hint, func(r rune) bool {
		return r == '/' || r == ',' || r == ';'
	})
	if len(fields) == 0 {
		return nil
	}
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		if value := strings.TrimSpace(field); value != "" {
			out = append(out, value)
		}
	}
	return out
}

func normalizeKeyboard(input string) string {
	var out strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(input)) {
		if r >= 'a' && r <= 'z' {
			out.WriteRune(r)
			continue
		}
		if unicode.IsDigit(r) {
			out.WriteRune(r)
		}
	}
	return out.String()
}

func normalizeTypingSequence(input string) string {
	var out strings.Builder
	lastSpace := true
	for _, r := range strings.ToLower(strings.TrimSpace(input)) {
		switch {
		case r >= 'a' && r <= 'z':
			out.WriteRune(r)
			lastSpace = false
		case unicode.IsDigit(r):
			out.WriteRune(r)
			lastSpace = false
		case unicode.IsSpace(r) || r == '-' || r == '_':
			if !lastSpace {
				out.WriteRune(' ')
				lastSpace = true
			}
		}
	}
	return strings.TrimSpace(out.String())
}

func isVowel(b byte) bool {
	return b == 'a' || b == 'i' || b == 'u' || b == 'e' || b == 'o'
}

func isConsonant(b byte) bool {
	return b >= 'a' && b <= 'z' && !isVowel(b)
}

func BasicRows() []Row {
	return []Row{
		{Label: "vowel", Cells: basicCells("vowel", []string{"a", "i", "u", "e", "o"})},
		{Label: "k", Cells: basicCells("k", []string{"ka", "ki", "ku", "ke", "ko"})},
		{Label: "s", Cells: basicCells("s", []string{"sa", "shi", "su", "se", "so"})},
		{Label: "t", Cells: basicCells("t", []string{"ta", "chi", "tsu", "te", "to"})},
		{Label: "n", Cells: basicCells("n", []string{"na", "ni", "nu", "ne", "no"})},
		{Label: "h", Cells: basicCells("h", []string{"ha", "hi", "fu", "he", "ho"})},
		{Label: "m", Cells: basicCells("m", []string{"ma", "mi", "mu", "me", "mo"})},
		{Label: "y", Cells: basicCells("y", []string{"ya", "yu", "yo"})},
		{Label: "r", Cells: basicCells("r", []string{"ra", "ri", "ru", "re", "ro"})},
		{Label: "w", Cells: basicCells("w", []string{"wa", "wo"})},
		{Label: "n'", Cells: []Cell{{Row: "n'", Vowel: "", Romaji: "n", Hiragana: "ん", Katakana: "ン"}}},
	}
}

func basicCells(row string, romaji []string) []Cell {
	cells := make([]Cell, 0, len(romaji))
	for _, token := range romaji {
		hiragana := keyboardKana[token]
		cells = append(cells, Cell{
			Row:      row,
			Vowel:    tokenVowel(token),
			Romaji:   token,
			Hiragana: hiragana,
			Katakana: ToKatakana(hiragana),
		})
	}
	return cells
}

func tokenVowel(token string) string {
	for i := len(token) - 1; i >= 0; i-- {
		if isVowel(token[i]) {
			return string(token[i])
		}
	}
	return ""
}

var keyboardKana = map[string]string{
	"a": "あ", "i": "い", "u": "う", "e": "え", "o": "お",

	"ka": "か", "ki": "き", "ku": "く", "ke": "け", "ko": "こ",
	"ga": "が", "gi": "ぎ", "gu": "ぐ", "ge": "げ", "go": "ご",

	"sa": "さ", "shi": "し", "si": "し", "su": "す", "se": "せ", "so": "そ",
	"za": "ざ", "ji": "じ", "zi": "じ", "zu": "ず", "ze": "ぜ", "zo": "ぞ",

	"ta": "た", "chi": "ち", "ti": "ち", "tsu": "つ", "tu": "つ", "te": "て", "to": "と",
	"da": "だ", "di": "ぢ", "du": "づ", "de": "で", "do": "ど",

	"na": "な", "ni": "に", "nu": "ぬ", "ne": "ね", "no": "の",
	"ha": "は", "hi": "ひ", "fu": "ふ", "hu": "ふ", "he": "へ", "ho": "ほ",
	"ba": "ば", "bi": "び", "bu": "ぶ", "be": "べ", "bo": "ぼ",
	"pa": "ぱ", "pi": "ぴ", "pu": "ぷ", "pe": "ぺ", "po": "ぽ",
	"ma": "ま", "mi": "み", "mu": "む", "me": "め", "mo": "も",
	"ya": "や", "yu": "ゆ", "yo": "よ",
	"ra": "ら", "ri": "り", "ru": "る", "re": "れ", "ro": "ろ",
	"wa": "わ", "wo": "を",

	"kya": "きゃ", "kyu": "きゅ", "kyo": "きょ",
	"gya": "ぎゃ", "gyu": "ぎゅ", "gyo": "ぎょ",
	"sha": "しゃ", "shu": "しゅ", "sho": "しょ",
	"sya": "しゃ", "syu": "しゅ", "syo": "しょ",
	"ja": "じゃ", "ju": "じゅ", "jo": "じょ",
	"jya": "じゃ", "jyu": "じゅ", "jyo": "じょ",
	"cha": "ちゃ", "chu": "ちゅ", "cho": "ちょ",
	"tya": "ちゃ", "tyu": "ちゅ", "tyo": "ちょ",
	"nya": "にゃ", "nyu": "にゅ", "nyo": "にょ",
	"hya": "ひゃ", "hyu": "ひゅ", "hyo": "ひょ",
	"bya": "びゃ", "byu": "びゅ", "byo": "びょ",
	"pya": "ぴゃ", "pyu": "ぴゅ", "pyo": "ぴょ",
	"mya": "みゃ", "myu": "みゅ", "myo": "みょ",
	"rya": "りゃ", "ryu": "りゅ", "ryo": "りょ",
}

var canonicalKanaKeyboard = buildCanonicalKanaKeyboard()

func buildCanonicalKanaKeyboard() map[string]string {
	out := make(map[string]string, len(canonicalKeyboardTokens)*2+2)
	for _, token := range canonicalKeyboardTokens {
		if kana, ok := keyboardKana[token]; ok {
			out[kana] = token
			out[ToKatakana(kana)] = token
		}
	}
	out["ん"] = "n"
	out["ン"] = "n"
	return out
}

var canonicalKeyboardTokens = []string{
	"a", "i", "u", "e", "o",

	"ka", "ki", "ku", "ke", "ko",
	"ga", "gi", "gu", "ge", "go",

	"sa", "shi", "su", "se", "so",
	"za", "ji", "zu", "ze", "zo",

	"ta", "chi", "tsu", "te", "to",
	"da", "di", "du", "de", "do",

	"na", "ni", "nu", "ne", "no",
	"ha", "hi", "fu", "he", "ho",
	"ba", "bi", "bu", "be", "bo",
	"pa", "pi", "pu", "pe", "po",
	"ma", "mi", "mu", "me", "mo",
	"ya", "yu", "yo",
	"ra", "ri", "ru", "re", "ro",
	"wa", "wo",

	"kya", "kyu", "kyo",
	"gya", "gyu", "gyo",
	"sha", "shu", "sho",
	"ja", "ju", "jo",
	"cha", "chu", "cho",
	"nya", "nyu", "nyo",
	"hya", "hyu", "hyo",
	"bya", "byu", "byo",
	"pya", "pyu", "pyo",
	"mya", "myu", "myo",
	"rya", "ryu", "ryo",
}
