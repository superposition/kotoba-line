package content

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

type ValidationSeverity string

const (
	ValidationWarning ValidationSeverity = "warning"
	ValidationError   ValidationSeverity = "error"
)

type ValidationIssue struct {
	Severity ValidationSeverity `json:"severity"`
	Code     string             `json:"code"`
	Message  string             `json:"message"`
	Path     string             `json:"path,omitempty"`
	ID       string             `json:"id,omitempty"`
}

type ValidationReport struct {
	Issues []ValidationIssue `json:"issues"`
}

func LoadFile(path string) (*Library, ValidationReport, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, ValidationReport{}, err
	}
	defer f.Close()

	return LoadJSON(f)
}

func LoadJSON(r io.Reader) (*Library, ValidationReport, error) {
	var library Library
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&library); err != nil {
		return nil, ValidationReport{}, err
	}

	report := ValidateLibrary(&library)
	return &library, report, nil
}

func ValidateLibrary(library *Library) ValidationReport {
	var report ValidationReport
	if library == nil {
		report.add(ValidationError, "missing_library", "content library is nil", "", "")
		return report
	}

	cardIDs := make(map[string]struct{}, len(library.Cards))
	for i := range library.Cards {
		card := &library.Cards[i]
		path := fmt.Sprintf("cards[%d]", i)

		if blank(card.ID) {
			report.add(ValidationError, "missing_card_id", "card id is required", path+".id", "")
		} else if _, exists := cardIDs[card.ID]; exists {
			report.add(ValidationError, "duplicate_card_id", "card id must be unique", path+".id", card.ID)
		} else {
			cardIDs[card.ID] = struct{}{}
		}

		if blank(card.Text) {
			report.add(ValidationError, "missing_card_text", "card text is required", path+".text", card.ID)
		}
		if blank(string(card.Type)) {
			report.add(ValidationError, "missing_card_type", "card type is required", path+".type", card.ID)
		}
		if blank(card.Meaning) {
			report.add(ValidationWarning, "missing_meaning", "card meaning should be curated", path+".meaning", card.ID)
			card.NeedsReview = true
		}
		if blank(card.Reading.Kana) {
			card.Playable = false
			card.NeedsReview = true
			report.add(ValidationWarning, "missing_kana", "card has no curated kana reading and is not playable", path+".reading.kana", card.ID)
		}
	}

	documentIDs := make(map[string]map[string]struct{}, len(library.Documents))
	for i, document := range library.Documents {
		path := fmt.Sprintf("documents[%d]", i)
		if blank(document.ID) {
			report.add(ValidationError, "missing_document_id", "document id is required", path+".id", "")
			continue
		}
		if _, exists := documentIDs[document.ID]; exists {
			report.add(ValidationError, "duplicate_document_id", "document id must be unique", path+".id", document.ID)
			continue
		}

		sections := make(map[string]struct{}, len(document.Sections))
		for sectionIndex, section := range document.Sections {
			sectionPath := fmt.Sprintf("%s.sections[%d]", path, sectionIndex)
			if blank(section.ID) {
				report.add(ValidationError, "missing_section_id", "document section id is required", sectionPath+".id", document.ID)
				continue
			}
			if _, exists := sections[section.ID]; exists {
				report.add(ValidationError, "duplicate_section_id", "document section id must be unique within a document", sectionPath+".id", section.ID)
				continue
			}
			sections[section.ID] = struct{}{}
		}
		documentIDs[document.ID] = sections
	}

	levelIDs := make(map[string]struct{}, len(library.Levels))
	for i, level := range library.Levels {
		path := fmt.Sprintf("levels[%d]", i)
		if blank(level.ID) {
			report.add(ValidationError, "missing_level_id", "level id is required", path+".id", "")
		} else if _, exists := levelIDs[level.ID]; exists {
			report.add(ValidationError, "duplicate_level_id", "level id must be unique", path+".id", level.ID)
		} else {
			levelIDs[level.ID] = struct{}{}
		}

		if !blank(level.DocumentID) {
			sections, exists := documentIDs[level.DocumentID]
			if !exists {
				report.add(ValidationError, "unknown_level_document", "level references an unknown document", path+".document_id", level.ID)
			} else if !blank(level.SectionID) {
				if _, exists := sections[level.SectionID]; !exists {
					report.add(ValidationError, "unknown_level_section", "level references an unknown document section", path+".section_id", level.ID)
				}
			}
		}

		reportUnknownCards(&report, path+".card_ids", level.ID, level.CardIDs, cardIDs)
		reportUnknownCards(&report, path+".required_card_ids", level.ID, level.RequiredCardIDs, cardIDs)
	}

	campaignIDs := make(map[string]struct{}, len(library.Campaigns))
	for i, campaign := range library.Campaigns {
		path := fmt.Sprintf("campaigns[%d]", i)
		if blank(campaign.ID) {
			report.add(ValidationError, "missing_campaign_id", "campaign id is required", path+".id", "")
		} else if _, exists := campaignIDs[campaign.ID]; exists {
			report.add(ValidationError, "duplicate_campaign_id", "campaign id must be unique", path+".id", campaign.ID)
		} else {
			campaignIDs[campaign.ID] = struct{}{}
		}

		for _, documentID := range campaign.DocumentIDs {
			if _, exists := documentIDs[documentID]; !exists {
				report.add(ValidationError, "unknown_campaign_document", "campaign references an unknown document", path+".document_ids", campaign.ID)
			}
		}
		for _, levelID := range campaign.LevelIDs {
			if _, exists := levelIDs[levelID]; !exists {
				report.add(ValidationError, "unknown_campaign_level", "campaign references an unknown level", path+".level_ids", campaign.ID)
			}
		}
		if !blank(campaign.StartLevelID) {
			if _, exists := levelIDs[campaign.StartLevelID]; !exists {
				report.add(ValidationError, "unknown_campaign_start_level", "campaign start level is unknown", path+".start_level_id", campaign.ID)
			}
		}
	}

	return report
}

func (r ValidationReport) HasErrors() bool {
	for _, issue := range r.Issues {
		if issue.Severity == ValidationError {
			return true
		}
	}
	return false
}

func (r ValidationReport) HasIssue(code string) bool {
	for _, issue := range r.Issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}

func reportUnknownCards(report *ValidationReport, path string, ownerID string, cardIDs []string, known map[string]struct{}) {
	for _, cardID := range cardIDs {
		if _, exists := known[cardID]; !exists {
			report.add(ValidationError, "unknown_card", "reference points to an unknown card", path, ownerID)
		}
	}
}

func (r *ValidationReport) add(severity ValidationSeverity, code, message, path, id string) {
	r.Issues = append(r.Issues, ValidationIssue{
		Severity: severity,
		Code:     code,
		Message:  message,
		Path:     path,
		ID:       id,
	})
}

func blank(value string) bool {
	return strings.TrimSpace(value) == ""
}
