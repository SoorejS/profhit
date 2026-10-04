package services

import (
	"errors"
	"net/url"
	"profhit-backend/models"
	"strings"
	"time"
)

type EditorialReview struct {
	Current        bool   `json:"current"`
	FutureOutcome  bool   `json:"future_outcome"`
	Objective      bool   `json:"objective"`
	TrustedSource  bool   `json:"trusted_source"`
	SensibleCutoff bool   `json:"sensible_cutoff"`
	Interesting    bool   `json:"interesting"`
	Rationale      string `json:"rationale"`
}

// Interest requires an accountable editorial decision; it cannot be inferred from a row count.
func ValidateEditorialReview(m models.Market, r EditorialReview, now time.Time) error {
	if !r.Current || !r.FutureOutcome || !r.Objective || !r.TrustedSource || !r.SensibleCutoff || !r.Interesting || len(strings.TrimSpace(r.Rationale)) < 30 || len(r.Rationale) > 2000 {
		return errors.New("six editorial checks and a specific relevance rationale are required")
	}
	if len(strings.TrimSpace(m.Description)) < 30 || m.LockTime == nil || !m.LockTime.After(now) || m.ResolutionTime == nil || m.ResolutionTime.Before(*m.LockTime) || !ApprovedResultURL(m.Category, m.ResolutionSource) {
		return errors.New("provide real context, a future cutoff, resolution time and trusted source")
	}
	text := strings.ToLower(m.Title + " " + m.Description)
	if strings.Contains(text, "who wins on ") || strings.Contains(text, "curated from the linked live official source") || strings.Contains(text, "team a at team b") {
		return errors.New("generic fixture filler must be rewritten with real context")
	}
	return nil
}

// Context can come from the public broadcaster; outcome evidence still uses the category allowlist.
func ApprovedEditorialSource(category, raw string) bool {
	if ApprovedResultURL(category, raw) {
		return true
	}
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.User == nil && u.Port() == "" && (u.Hostname() == "newsonair.gov.in" || u.Hostname() == "www.newsonair.gov.in" || u.Hostname() == "pib.gov.in" || u.Hostname() == "www.pib.gov.in")
}
