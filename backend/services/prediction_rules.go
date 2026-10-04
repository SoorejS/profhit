package services

import (
	"encoding/json"
	"errors"
	"math"
	"net/url"
	"profhit-backend/models"
	"strconv"
	"strings"
)

var CategoryPayouts = map[string][3]int{
	"Weather": {20, 50, 120}, "Sports": {25, 60, 200}, "Politics": {30, 80, 250},
	"Entertainment": {25, 70, 150}, "Financial Markets": {20, 80, 300}, "Wild Card": {40, 100, 400},
	"Technology": {40, 100, 400}, "Geopolitics": {40, 100, 400},
}

var CategoryTypes = map[string][3]string{
	"Weather": {"binary", "range", "exact"}, "Sports": {"winner", "range", "score"},
	"Politics": {"winner", "margin", "seat_count"}, "Entertainment": {"winner", "top3", "range"},
	"Financial Markets": {"direction", "percent_range", "closest_price"}, "Wild Card": {"binary", "multi_choice", "closest_text"},
	"Technology": {"binary", "multi_choice", "closest_text"}, "Geopolitics": {"binary", "multi_choice", "closest_text"},
}

var resultDomains = map[string][]string{
	"Weather":           {"openweathermap.org", "imd.gov.in", "weather.gov"},
	"Sports":            {"cricapi.com", "cricketdata.org", "sportmonks.com", "espn.com", "espncricinfo.com", "bcci.tv", "windiescricket.com", "icc-cricket.com", "formula1.com", "fia.com"},
	"Politics":          {"eci.gov.in", "results.eci.gov.in", "fec.gov", "elections.tn.gov.in"},
	"Entertainment":     {"bollywoodhungama.com", "boxofficeindia.com", "oscars.org", "grammy.com", "nobelprize.org", "nobelpeaceprize.org"},
	"Financial Markets": {"apple.com", "microsoft.com", "nseindia.com", "bseindia.com", "coingecko.com", "sec.gov", "rbi.org.in", "federalreserve.gov", "imf.org", "worldbank.org", "tesla.com", "tsmc.com", "infosys.com", "tcs.com", "netflix.net", "amd.com", "nvidia.com", "abc.xyz"},
	"Wild Card":         {"mausam.imd.gov.in", "eci.gov.in", "nseindia.com", "bseindia.com", "oscars.org", "nasa.gov", "isro.gov.in", "nobelprize.org", "nobelpeaceprize.org"},
	"Technology":        {"apple.com", "blog.google", "google.com", "microsoft.com", "github.com", "nasa.gov", "openai.com", "anthropic.com", "nvidia.com", "meta.com", "about.fb.com", "sec.gov", "python.org", "w3.org", "ubuntu.com", "fedoraproject.org", "fedorapeople.org", "abc.xyz"},
	"Geopolitics":       {"un.org", "consilium.europa.eu", "nato.int", "state.gov", "mea.gov.in", "kremlin.ru", "president.gov.ua", "whitehouse.gov", "ustr.gov", "g20.org", "unfccc.int", "imf.org", "worldbank.org"},
}

func ApprovedResultURL(category, raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	for _, domain := range resultDomains[category] {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return true
		}
	}
	return false
}

func EvidenceMatchesSource(m models.Market, raw string) bool {
	if !ApprovedResultURL(m.Category, raw) || !ApprovedResultURL(m.Category, m.ResolutionSource) {
		return false
	}
	configured, _ := url.Parse(m.ResolutionSource)
	evidence, _ := url.Parse(raw)
	return strings.TrimPrefix(strings.ToLower(configured.Hostname()), "www.") == strings.TrimPrefix(strings.ToLower(evidence.Hostname()), "www.")
}

func ConfigurePrediction(m *models.Market) error {
	if m.ResultSpec != "" {
		var spec WeatherResultSpec
		if json.Unmarshal([]byte(m.ResultSpec), &spec) != nil || spec.Validate(*m) != nil || m.ResolutionRule != spec.Rule() || m.ResolutionSource != spec.EvidenceURL() || m.ResultApprovedBy == 0 || m.ResolutionTime == nil || !m.ResolutionTime.Equal(spec.ObservationFrom) {
			return errors.New("provider result settings do not match the published rule, source or time")
		}
	}
	tier := map[string]int{"Easy": 0, "Medium": 1, "Hard": 2}
	idx, ok := tier[m.Difficulty]
	if !ok {
		return errors.New("invalid difficulty")
	}
	payouts, ok := CategoryPayouts[m.Category]
	if !ok {
		return errors.New("invalid category")
	}
	m.Payout = payouts[idx]
	if m.EntryCoins < 0 || m.EntryCoins >= m.Payout {
		return errors.New("virtual coin entry cost must be non-negative and less than the correct-answer reward")
	}
	expected := CategoryTypes[m.Category][idx]
	if m.PredictionType == "binary" && idx == 0 && (expected == "winner" || expected == "direction") {
		expected = "binary"
	}
	if m.PredictionType != "" && m.PredictionType != expected {
		return errors.New("prediction type does not match category and difficulty")
	}
	m.PredictionType = expected
	var options []string
	if json.Unmarshal([]byte(m.Options), &options) != nil {
		return errors.New("options must be a JSON array")
	}
	if (expected == "binary" && len(options) != 2) || (expected == "multi_choice" && len(options) != 4) || (expected == "top3" && len(options) < 3) {
		return errors.New("provide two binary options, four multi-choice options, or at least three top-three nominees")
	}
	if !ApprovedResultURL(m.Category, m.ResolutionSource) {
		return errors.New("provide an approved HTTPS result source for this category")
	}
	if strings.TrimSpace(m.ResolutionRule) == "" {
		return errors.New("a measurable resolution rule and units are required")
	}
	if m.PredictionType == "range" && (m.RangeWidth <= 0 || math.IsInf(m.RangeWidth, 0) || math.IsNaN(m.RangeWidth)) {
		return errors.New("range predictions require a positive maximum interval width")
	}
	if m.Category == "Financial Markets" && m.Difficulty == "Medium" && m.RangeWidth > 2 {
		return errors.New("financial percentage interval must be at most 2 percentage points wide")
	}
	m.EvidenceURL = ""
	m.ObservedAt = nil
	return nil
}

func numeric(raw string) (float64, error) {
	n, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || math.IsNaN(n) || math.IsInf(n, 0) || math.Abs(n) > 1e12 {
		return 0, errors.New("enter a finite number within the supported bounds")
	}
	return n, nil
}

// Values are canonicalized before storage. Free text is restricted to measurable numeric answers.
func ValidatePredictionValue(m models.Market, raw string, outcome bool) (string, error) {
	var options []string
	_ = json.Unmarshal([]byte(m.Options), &options)
	switch m.PredictionType {
	case "", "binary", "winner", "direction", "multi_choice":
		for _, option := range options {
			if option == raw {
				return raw, nil
			}
		}
		return "", errors.New("choose a declared option")
	case "range":
		if outcome {
			n, err := numeric(raw)
			return strconv.FormatFloat(n, 'f', -1, 64), err
		}
		var bounds []float64
		if json.Unmarshal([]byte(raw), &bounds) != nil || len(bounds) != 2 {
			return "", errors.New("enter a range as [minimum,maximum]")
		}
		if math.IsNaN(bounds[0]) || math.IsNaN(bounds[1]) || math.IsInf(bounds[0], 0) || math.IsInf(bounds[1], 0) || math.Abs(bounds[0]) > 1e12 || math.Abs(bounds[1]) > 1e12 || bounds[1] < bounds[0] || bounds[1]-bounds[0] > m.RangeWidth {
			return "", errors.New("range exceeds the published width or bounds")
		}
		data, _ := json.Marshal(bounds)
		return string(data), nil
	case "top3":
		var choices []string
		if json.Unmarshal([]byte(raw), &choices) != nil || len(choices) != 3 {
			return "", errors.New("enter exactly three unique declared nominees as a JSON array")
		}
		seen := map[string]bool{}
		for _, v := range choices {
			valid := false
			for _, opt := range options {
				if v == opt {
					valid = true
				}
			}
			if !valid || seen[v] {
				return "", errors.New("invalid or repeated nominee")
			}
			seen[v] = true
		}
		data, _ := json.Marshal(choices)
		return string(data), nil
	case "score":
		if strings.HasPrefix(raw, "[") {
			var score []int64
			if json.Unmarshal([]byte(raw), &score) != nil || len(score) != 2 || score[0] < 0 || score[1] < 0 || score[0] > 100000 || score[1] > 100000 {
				return "", errors.New("score must be a nonnegative integer or [home,away]")
			}
			data, _ := json.Marshal(score)
			return string(data), nil
		}
		n, err := numeric(raw)
		if err != nil || n < 0 || n != math.Trunc(n) {
			return "", errors.New("run total must be a nonnegative integer")
		}
		return strconv.FormatFloat(n, 'f', -1, 64), nil
	case "exact", "margin", "percent_range", "seat_count", "closest_price", "closest_text":
		n, err := numeric(raw)
		if err != nil {
			return "", err
		}
		if m.PredictionType == "seat_count" && (n < 0 || n != math.Trunc(n)) {
			return "", errors.New("seat count must be a nonnegative integer")
		}
		if m.PredictionType == "margin" && (n < 0 || n > 100) {
			return "", errors.New("margin must be between 0 and 100 percent")
		}
		return strconv.FormatFloat(n, 'f', -1, 64), nil
	default:
		return "", errors.New("unsupported prediction type")
	}
}

// Closest answers share the win when distances tie; payout is fixed, never stake-dependent.
func PredictionWinners(m models.Market, actual string, predictions []models.PredictionSubmission) (map[uint]bool, error) {
	value, err := ValidatePredictionValue(m, actual, true)
	if err != nil {
		return nil, err
	}
	wins := map[uint]bool{}
	best := math.Inf(1)
	for _, pred := range predictions {
		correct := pred.Choice == value
		switch m.PredictionType {
		case "range":
			var bounds []float64
			n, _ := numeric(value)
			if json.Unmarshal([]byte(pred.Choice), &bounds) == nil && len(bounds) == 2 {
				correct = n >= bounds[0] && n <= bounds[1]
			}
		case "margin":
			n, _ := numeric(value)
			guess, e := numeric(pred.Choice)
			correct = e == nil && math.Abs(n-guess) <= 5
		case "percent_range":
			n, _ := numeric(value)
			guess, e := numeric(pred.Choice)
			correct = e == nil && math.Abs(n-guess) <= 1
		case "top3":
			var a, b []string
			_ = json.Unmarshal([]byte(value), &a)
			_ = json.Unmarshal([]byte(pred.Choice), &b)
			set := map[string]bool{}
			for _, v := range a {
				set[v] = true
			}
			correct = len(b) == 3
			for _, v := range b {
				correct = correct && set[v]
			}
		case "closest_price", "closest_text":
			n, _ := numeric(value)
			guess, e := numeric(pred.Choice)
			if e != nil {
				continue
			}
			distance := math.Abs(n - guess)
			if distance < best {
				best = distance
				wins = map[uint]bool{pred.ID: true}
			} else if distance == best {
				wins[pred.ID] = true
			}
			continue
		}
		wins[pred.ID] = correct
	}
	return wins, nil
}
