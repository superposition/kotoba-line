package state

import (
	"fmt"

	"github.com/superposition/kotoba-line/internal/content"
)

type ankiVerbSeed struct {
	Text    string
	Kana    string
	Romaji  string
	Meaning string
	Notes   string
}

func ankiN5VerbBoostLessons() []lessonSeed {
	groups := []struct {
		Number int
		Title  string
		Cards  []ankiVerbSeed
	}{
		{1, "Reading And Daily Motion", []ankiVerbSeed{
			{"読みます", "よみます", "yomimasu", "read", "from local Anki deck: JLPT N5 Beginner - 70 Practical Verbs"},
			{"浴びます", "あびます", "abimasu", "take a shower", "deck front included shower phrasing; imported as the corrected verb 浴びます"},
			{"上げます", "あげます", "agemasu", "raise something", "transitive ichidan verb"},
			{"開けます", "あけます", "akemasu", "open something", "transitive ichidan verb"},
			{"洗います", "あらいます", "araimasu", "wash something", "godan verb"},
			{"あります", "あります", "arimasu", "exist; have for inanimate things", "existence verb"},
			{"歩きます", "あるきます", "arukimasu", "walk", "godan verb"},
			{"遊びます", "あそびます", "asobimasu", "play", "godan verb"},
			{"会います", "あいます", "aimasu", "meet", "godan verb"},
			{"勉強します", "べんきょうします", "benkyou shimasu", "study", "suru verb"},
		}},
		{2, "Speaking And Leaving", []ankiVerbSeed{
			{"します", "します", "shimasu", "do", "irregular verb"},
			{"話します", "はなします", "hanashimasu", "speak; talk", "godan verb"},
			{"忘れます", "わすれます", "wasuremasu", "forget", "ichidan verb"},
			{"出かけます", "でかけます", "dekakemasu", "go out; leave", "ichidan verb"},
			{"電話をします", "でんわをします", "denwa o shimasu", "call; telephone", "suru phrase"},
			{"降ります", "ふります", "furimasu", "fall, as rain or snow", "weather verb"},
			{"分かります", "わかります", "wakarimasu", "understand", "godan verb"},
			{"歌います", "うたいます", "utaimasu", "sing", "godan verb"},
			{"売ります", "うります", "urimasu", "sell", "godan verb"},
			{"生まれます", "うまれます", "umaremasu", "be born", "ichidan verb"},
		}},
		{3, "Making And Using", []ankiVerbSeed{
			{"作ります", "つくります", "tsukurimasu", "make", "godan verb"},
			{"付けます", "つけます", "tsukemasu", "turn on; attach", "ichidan verb"},
			{"使います", "つかいます", "tsukaimasu", "use", "godan verb"},
			{"飛びます", "とびます", "tobimasu", "fly", "godan verb"},
			{"立ちます", "たちます", "tachimasu", "stand", "godan verb"},
			{"頼みます", "たのみます", "tanomimasu", "ask; request", "godan verb"},
			{"食べます", "たべます", "tabemasu", "eat", "ichidan verb"},
			{"座ります", "すわります", "suwarimasu", "sit", "godan verb"},
			{"吸います", "すいます", "suimasu", "smoke; inhale", "godan verb"},
			{"住みます", "すみます", "sumimasu", "live in a place", "godan verb"},
		}},
		{4, "Suru Action Pack", []ankiVerbSeed{
			{"掃除します", "そうじします", "souji shimasu", "clean", "suru verb"},
			{"質問します", "しつもんします", "shitsumon shimasu", "ask a question", "suru verb"},
			{"仕事します", "しごとします", "shigoto shimasu", "work", "suru verb"},
			{"始まります", "はじまります", "hajimarimasu", "begin", "intransitive godan verb"},
			{"散歩します", "さんぽします", "sanpo shimasu", "go for a walk", "suru verb"},
			{"料理します", "りょうりします", "ryouri shimasu", "cook", "suru verb"},
			{"旅行します", "りょこうします", "ryokou shimasu", "travel", "suru verb"},
			{"練習します", "れんしゅうします", "renshuu shimasu", "practice", "suru verb"},
			{"結婚します", "けっこんします", "kekkon shimasu", "marry", "suru verb"},
			{"知ります", "しります", "shirimasu", "know; learn", "godan verb"},
		}},
		{5, "Weather Body And Work", []ankiVerbSeed{
			{"閉めます", "しめます", "shimemasu", "close something", "transitive ichidan verb"},
			{"閉まります", "しまります", "shimarimasu", "close; be closed", "intransitive godan verb"},
			{"咲きます", "さきます", "sakimasu", "bloom", "godan verb"},
			{"泳ぎます", "およぎます", "oyogimasu", "swim", "godan verb"},
			{"教えます", "おしえます", "oshiemasu", "teach", "ichidan verb"},
			{"晴れます", "はれます", "haremasu", "be sunny", "ichidan verb"},
			{"走ります", "はしります", "hashirimasu", "run", "godan verb"},
			{"働きます", "はたらきます", "hatarakimasu", "work", "godan verb"},
			{"弾きます", "ひきます", "hikimasu", "play an instrument", "godan verb"},
			{"行きます", "いきます", "ikimasu", "go", "godan verb"},
		}},
		{6, "Core Life Verbs", []ankiVerbSeed{
			{"います", "います", "imasu", "exist for animate things", "usually written in kana at this level"},
			{"書きます", "かきます", "kakimasu", "write; draw", "godan verb"},
			{"買います", "かいます", "kaimasu", "buy", "godan verb"},
			{"聞きます", "ききます", "kikimasu", "hear; listen; ask", "godan verb"},
			{"曇ります", "くもります", "kumorimasu", "be cloudy", "godan verb"},
			{"来ます", "きます", "kimasu", "come", "irregular verb"},
			{"待ちます", "まちます", "machimasu", "wait", "godan verb"},
			{"見ます", "みます", "mimasu", "see; look; watch", "ichidan verb"},
			{"習います", "ならいます", "naraimasu", "learn; be taught", "godan verb"},
			{"寝ます", "ねます", "nemasu", "sleep; go to bed", "ichidan verb"},
		}},
		{7, "More Everyday Actions", []ankiVerbSeed{
			{"登ります", "のぼります", "noborimasu", "climb", "godan verb"},
			{"飲みます", "のみます", "nomimasu", "drink", "godan verb"},
			{"覚えます", "おぼえます", "oboemasu", "remember", "ichidan verb"},
			{"起きます", "おきます", "okimasu", "get up; wake up", "ichidan verb"},
			{"置きます", "おきます", "okimasu", "put; place", "godan verb"},
			{"あげます", "あげます", "agemasu", "give", "usually written in kana at this level"},
			{"持ちます", "もちます", "mochimasu", "hold; have", "godan verb"},
			{"もらいます", "もらいます", "moraimasu", "receive", "usually written in kana at this level"},
			{"塗ります", "ぬります", "nurimasu", "paint", "godan verb"},
			{"着ます", "きます", "kimasu", "wear", "ichidan verb; contrasts with 来ます"},
		}},
	}

	lessons := make([]lessonSeed, 0, len(groups))
	for _, group := range groups {
		lessons = append(lessons, lessonSeed{
			ID:            fmt.Sprintf("lesson-anki-n5-verbs-%02d", group.Number),
			Title:         fmt.Sprintf("Anki N5 Verb Boost.%02d - %s", group.Number, group.Title),
			Description:   "Extra N5 verb reps imported from the local Anki deck before the next Beginner 200 beach opens.",
			DocumentTitle: "Anki N5 Verb Boost",
			Cards:         ankiVerbCards(group.Number, group.Cards),
		})
	}
	return lessons
}

func ankiVerbCards(group int, seeds []ankiVerbSeed) []lessonCardSeed {
	cards := make([]lessonCardSeed, 0, len(seeds))
	for i, seed := range seeds {
		index := (group-1)*10 + i + 1
		cards = append(cards, lessonCardSeed{
			ID:         fmt.Sprintf("lesson-anki-n5-v%03d", index),
			Text:       seed.Text,
			Kanji:      seed.Text,
			Kana:       seed.Kana,
			RomajiHint: seed.Romaji,
			Meaning:    seed.Meaning,
			Type:       content.CardTypeWord,
			Notes:      seed.Notes,
			Tags:       fmt.Sprintf("sqlite|日-foundation|b200|anki|anki-n5-verbs|anki-n5-verbs-g%02d", group),
		})
	}
	return cards
}
