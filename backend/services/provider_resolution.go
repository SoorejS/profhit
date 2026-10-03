package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"profhit-backend/config"
	"profhit-backend/models"
	"strconv"
	"time"
)

type WeatherResultSpec struct {
	Provider         string    `json:"provider"`
	Latitude         float64   `json:"latitude"`
	Longitude        float64   `json:"longitude"`
	Metric           string    `json:"metric"`
	Threshold        float64   `json:"threshold"`
	ObservationFrom  time.Time `json:"observation_from"`
	ObservationUntil time.Time `json:"observation_until"`
}

func (s WeatherResultSpec) Validate(m models.Market) error {
	if s.Provider != "openweather" || m.Category != "Weather" || m.Difficulty != "Easy" || m.PredictionType != "binary" {
		return errors.New("automatic weather observations require an Easy Weather binary market")
	}
	if math.IsNaN(s.Latitude) || math.IsNaN(s.Longitude) || math.IsNaN(s.Threshold) || math.IsInf(s.Latitude, 0) || math.IsInf(s.Longitude, 0) || math.IsInf(s.Threshold, 0) || math.Abs(s.Latitude) > 90 || math.Abs(s.Longitude) > 180 || math.Abs(s.Threshold) > 1000 {
		return errors.New("invalid weather location or threshold")
	}
	if s.Metric != "temperature_c" && s.Metric != "rain_1h_mm" {
		return errors.New("supported measurements are temperature_c and rain_1h_mm")
	}
	if m.LockTime == nil || !s.ObservationFrom.After(*m.LockTime) || s.ObservationUntil.Sub(s.ObservationFrom) != 15*time.Minute {
		return errors.New("observation must follow the prediction cutoff within an exact 15-minute window")
	}
	return nil
}
func (s WeatherResultSpec) EvidenceURL() string {
	u, _ := url.Parse("https://api.openweathermap.org/data/2.5/weather")
	q := u.Query()
	q.Set("lat", strconv.FormatFloat(s.Latitude, 'f', -1, 64))
	q.Set("lon", strconv.FormatFloat(s.Longitude, 'f', -1, 64))
	q.Set("units", "metric")
	u.RawQuery = q.Encode()
	return u.String()
}
func (s WeatherResultSpec) Rule() string {
	return fmt.Sprintf("OpenWeather %s at coordinates %.5f,%.5f. Yes if strictly greater than %.4f; No otherwise, including equality. Use the first fetched provider observation whose dt falls in [%s,%s). Missing, stale or unavailable observations require manual review; no automatic guess or payout.", s.Metric, s.Latitude, s.Longitude, s.Threshold, s.ObservationFrom.UTC().Format(time.RFC3339), s.ObservationUntil.UTC().Format(time.RFC3339))
}

type ProviderOutcome struct {
	Outcome     string
	EvidenceURL string
	ObservedAt  time.Time
	Evidence    string
}

func FetchWeatherResult(ctx context.Context, client *http.Client, spec WeatherResultSpec, key string, now time.Time) (ProviderOutcome, error) {
	var result ProviderOutcome
	if key == "" {
		return result, errors.New("OpenWeather is not configured")
	}
	raw := spec.EvidenceURL()
	u, _ := url.Parse(raw)
	q := u.Query()
	q.Set("appid", key)
	u.RawQuery = q.Encode()
	data, err := providerRead(ctx, client, u.String())
	if err != nil {
		return result, err
	}
	var observation struct {
		DT    int64 `json:"dt"`
		Coord struct {
			Lat *float64 `json:"lat"`
			Lon *float64 `json:"lon"`
		} `json:"coord"`
		Main struct {
			Temperature *float64 `json:"temp"`
		} `json:"main"`
		Rain map[string]float64 `json:"rain"`
	}
	if json.Unmarshal(data, &observation) != nil {
		return result, errors.New("invalid weather observation")
	}
	observed := time.Unix(observation.DT, 0).UTC()
	if observation.Coord.Lat == nil || observation.Coord.Lon == nil || observation.DT <= 0 || observed.Before(spec.ObservationFrom) || !observed.Before(spec.ObservationUntil) || observed.After(now) || math.Abs(*observation.Coord.Lat-spec.Latitude) > 0.05 || math.Abs(*observation.Coord.Lon-spec.Longitude) > 0.05 {
		return result, errors.New("weather observation does not match published place/time")
	}
	value := 0.0
	if spec.Metric == "temperature_c" {
		if observation.Main.Temperature == nil {
			return result, errors.New("temperature missing")
		}
		value = *observation.Main.Temperature
	} else {
		var reported bool
		value, reported = observation.Rain["1h"]
		if !reported {
			return result, errors.New("rain measurement missing; manual evidence required")
		}
	}
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return result, errors.New("weather observation is invalid")
	}
	outcome := "No"
	if value > spec.Threshold {
		outcome = "Yes"
	}
	evidence, _ := json.Marshal(map[string]interface{}{"provider": "openweather", "observed_at": observed, "metric": spec.Metric, "value": value, "coordinates": observation.Coord, "source_url": raw})
	return ProviderOutcome{Outcome: outcome, EvidenceURL: raw, ObservedAt: observed, Evidence: string(evidence)}, nil
}
func resolveProviderMarkets() {
	if os.Getenv("OPENWEATHER_API_KEY") == "" {
		return
	}
	now := time.Now().UTC()
	var markets []models.Market
	if config.DB.Where("result_spec <> '' AND resolution_status IN ? AND resolution_time <= ? AND (next_resolution_attempt_at IS NULL OR next_resolution_attempt_at <= ?)", []string{"Locked", "Awaiting Resolution"}, now, now).Limit(10).Find(&markets).Error != nil {
		return
	}
	for _, market := range markets {
		var spec WeatherResultSpec
		if json.Unmarshal([]byte(market.ResultSpec), &spec) != nil || spec.Validate(market) != nil {
			continue
		}
		if !now.Before(spec.ObservationUntil) {
			_ = config.DB.Model(&market).Update("resolution_failure", "Observation window missed; manual evidence review required").Error
			continue
		}
		claim := config.DB.Model(&models.Market{}).Where("id = ? AND resolution_status IN ? AND (next_resolution_attempt_at IS NULL OR next_resolution_attempt_at <= ?)", market.ID, []string{"Locked", "Awaiting Resolution"}, now).Update("next_resolution_attempt_at", now.Add(5*time.Minute))
		if claim.Error != nil || claim.RowsAffected != 1 {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		result, err := FetchWeatherResult(ctx, providerClient(), spec, os.Getenv("OPENWEATHER_API_KEY"), now)
		cancel()
		if err == nil {
			_, err = SettleMarket(market.ID, ResolutionInput{Outcome: result.Outcome, EvidenceURL: result.EvidenceURL, ObservedAt: &result.ObservedAt, AdminID: market.ResultApprovedBy, ClientIP: "provider:openweather", ProviderEvidence: result.Evidence, ExpectedResultSpec: market.ResultSpec})
		}
		if err != nil {
			_ = config.DB.Model(&models.Market{}).Where("id = ? AND resolution_status IN ?", market.ID, []string{"Locked", "Awaiting Resolution"}).Update("resolution_failure", err.Error()).Error
		}
	}
}
