# Constitution Fixture Notes

This prep fixture seeds a small Kotoba Line campaign for:

- `preamble-1`: the first paragraph of the Japanese Constitution preamble.
- `article-1`: Chapter I, Article 1.

Official text lives in `content/documents/japan-constitution-preamble-article1.json`.
Learner hints live separately in
`content/campaigns/constitution-preamble-article1.json`.

## Source Policy

Primary source:

- e-Gov Law Search, `日本国憲法`, Law ID `321CONSTITUTION`
  - https://laws.e-gov.go.jp/law/321CONSTITUTION
  - https://laws.e-gov.go.jp/api/1/lawdata/321CONSTITUTION

Secondary rendering check:

- Japanese Law Translation, Japanese page
  - https://www.japaneselawtranslation.go.jp/en/laws/view/174/ja

The fixture preserves official orthography, including forms such as `わたつて`,
`ないやうに`, `あつて`, and `基く`. Do not modernize this text in document
fixtures. Add readings, romaji, glosses, chunking, and gameplay prerequisites in
campaign fixtures instead.

## Review Flags

The current learner readings are prep hints, not final playable cards. Keep
`needs_native_review` until a Japanese reviewer confirms:

- Historical kana readings are presented correctly for learners.
- Topic-particle `は` hints are separated from source text.
- Legal-register glosses such as `主権`, `存する`, `総意`, and `詔勅` are accurate
  enough for the target learner level.
