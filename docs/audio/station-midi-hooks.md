# Station MIDI Hooks

Station metadata lives in `content/stations/catalog.json`.

Each station can declare optional `midi_hooks` with a local `path`. Loading the
station catalog never requires the MIDI file to exist, and local registration
through `station.Catalog.RegisterLocalMIDI` records hooks as optional.

Runtime playback is planned through `internal/audio.PlanStation`:

- local mode uses a MIDI file only when MIDI is enabled and the file exists;
- missing MIDI files fall back to a silent visual pulse;
- missing MIDI tooling falls back to a silent visual pulse;
- Railway mode always returns a visual-pulse-only plan.

No copyrighted MIDI files should be committed. Use `assets/midi/` only for
rights-cleared local files or placeholder documentation.
