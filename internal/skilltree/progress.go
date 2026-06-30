package skilltree

type Status string

const (
	StatusLocked     Status = "locked"
	StatusDiscovered Status = "discovered"
	StatusTraining   Status = "training"
	StatusMastered   Status = "mastered"
)

const MasteryStreak = 1

// Progress is the state adapter shape expected from a durable state package.
// Until that package exists, tests and callers can use MapProgress.
type Progress struct {
	Status Status
	Streak int
}

type ProgressProvider interface {
	CardProgress(cardID string) Progress
}

type MapProgress map[string]Progress

func (m MapProgress) CardProgress(cardID string) Progress {
	if m == nil {
		return Progress{}
	}
	return m[cardID]
}

func Hit(progress Progress) Progress {
	progress = normalize(progress)
	if progress.Status == StatusMastered {
		return progress
	}

	streak := progress.Streak + 1
	if streak >= MasteryStreak {
		return Progress{Status: StatusMastered, Streak: MasteryStreak}
	}
	return Progress{Status: StatusTraining, Streak: streak}
}

func Miss(progress Progress) Progress {
	progress = normalize(progress)
	if progress.Status == StatusMastered {
		return Progress{Status: StatusMastered}
	}
	return Progress{Status: StatusDiscovered}
}
