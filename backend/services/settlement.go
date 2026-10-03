package services

import (
	"fmt"
	"gorm.io/gorm/clause"
	"net/http"
	"profhit-backend/config"
	"profhit-backend/models"
	"time"
)

type ResolutionInput struct {
	Winner             string     `json:"winner"`
	Outcome            string     `json:"outcome"`
	EvidenceURL        string     `json:"evidence_url"`
	ObservedAt         *time.Time `json:"observed_at"`
	AdminID            uint       `json:"-"`
	ClientIP           string     `json:"-"`
	ProviderEvidence   string     `json:"-"`
	ExpectedResultSpec string     `json:"-"`
}
type SettlementError struct {
	Status  int
	Message string
}

func (e *SettlementError) Error() string { return e.Message }

// Shared settlement transaction for reviewed manual evidence and deterministic providers.
func SettleMarket(id interface{}, input ResolutionInput) (map[string]interface{}, error) {
	tx := config.DB.Begin()
	defer tx.Rollback()
	var market models.Market
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&market).Error; err != nil {
		return nil, &SettlementError{Status: http.StatusNotFound, Message: "Market not found"}
	}

	// Allow resolution from Locked or Awaiting Resolution states.
	if input.ExpectedResultSpec != "" && market.ResultSpec != input.ExpectedResultSpec {
		return nil, &SettlementError{Status: 409, Message: "Result specification changed"}
	}
	// "Open" was a legacy value that never existed in the real lifecycle.
	resolvableStatuses := map[string]bool{"Locked": true, "Awaiting Resolution": true}
	if !resolvableStatuses[market.ResolutionStatus] {
		return nil, &SettlementError{Status: http.StatusBadRequest, Message: "Market must be Locked or Awaiting Resolution before it can be resolved. Current status: "}
	}

	// Normalise: prefer 'winner', fallback to 'outcome'
	correctOption := input.Winner
	if correctOption == "" {
		correctOption = input.Outcome
	}
	if correctOption == "" {
		return nil, &SettlementError{Status: http.StatusBadRequest, Message: "'winner' or 'outcome' field is required"}
	}

	// Validate that correct option is one of the market's actual options
	if !EvidenceMatchesSource(market, input.EvidenceURL) || input.ObservedAt == nil || input.ObservedAt.After(time.Now().UTC()) || (market.LockTime != nil && input.ObservedAt.Before(*market.LockTime)) {
		return nil, &SettlementError{Status: http.StatusBadRequest, Message: "Settlement requires approved-source evidence and an observation at or after the cutoff"}
	}
	canonical, err := ValidatePredictionValue(market, correctOption, true)
	if err != nil {
		return nil, &SettlementError{Status: 400, Message: err.Error()}
	}
	correctOption = canonical
	market.EvidenceURL = input.EvidenceURL
	market.ObservedAt = input.ObservedAt
	market.ResultEvidence = input.ProviderEvidence
	market.ResolutionFailure = ""

	// Capture the resolving admin's ID
	adminID := input.AdminID

	// ── Load all predictions for this market ────────────────────────────────
	var predictions []models.PredictionSubmission
	if err := tx.Where("market_id = ?", market.ID).Order("user_id ASC").Find(&predictions).Error; err != nil {
		return nil, &SettlementError{Status: http.StatusInternalServerError, Message: "Failed to load predictions"}
	}
	if len(predictions) > 0 {
		ids := []uint{}
		for _, p := range predictions {
			ids = append(ids, p.UserID)
		}
		var users []models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id IN ?", ids).Order("id asc").Find(&users).Error; err != nil {
			return nil, &SettlementError{Status: 500, Message: "Could not lock settlement accounts"}
		}
	}
	winners, err := PredictionWinners(market, correctOption, predictions)
	if err != nil {
		return nil, &SettlementError{Status: 400, Message: err.Error()}
	}

	// ── Process everything inside ONE atomic transaction ─────────────────────
	// CRITICAL FIX: CreditCoinsTx is called with the same tx object, ensuring
	// that if any payout fails, ALL changes (predictions + coins + market) roll back.

	winnerCount := 0
	loserCount := 0

	for i := range predictions {
		pred := &predictions[i]
		isCorrect := winners[pred.ID]
		pred.IsCorrect = &isCorrect

		if err := tx.Save(pred).Error; err != nil {
			tx.Rollback()
			return nil, &SettlementError{Status: http.StatusInternalServerError, Message: "Failed to update prediction record"}
		}

		if isCorrect {
			winnerCount++
			// CreditWalletTx runs INSIDE the same tx — rolls back if market save fails
			if err := CreditWalletTx(
				tx,
				pred.UserID,
				pred.Potential,
				models.TxTypePredictionWin,
				market.ID,
				"Won prediction on: "+market.Title,
				&adminID,
			); err != nil {
				tx.Rollback()
				return nil, &SettlementError{Status: http.StatusInternalServerError, Message: "Payout failed; no changes were committed"}
			}
			if err := tx.Create(&models.Notification{UserID: pred.UserID, Key: fmt.Sprintf("prediction:%d", pred.ID), Message: fmt.Sprintf("Correct prediction: +%d coins for %s", pred.Potential, market.Title)}).Error; err != nil {
				return nil, &SettlementError{Status: 500, Message: "Could not record reward notification"}
			}
		} else {
			loserCount++
		}
		if market.WeeklyChallengeID != nil {
			reward := 0
			score := 0
			if isCorrect {
				reward = pred.Potential
				score = 1
			}
			if err := tx.Model(&models.ChallengeParticipant{}).Where("challenge_id = ? AND user_id = ?", *market.WeeklyChallengeID, pred.UserID).Updates(map[string]interface{}{"score": score, "reward_won": reward}).Error; err != nil {
				return nil, &SettlementError{Status: 500, Message: "Could not settle challenge participation"}
			}
		}
	}
	if market.WeeklyChallengeID != nil {
		if err := tx.Model(&models.WeeklyChallenge{}).Where("id = ?", *market.WeeklyChallengeID).Update("status", "Completed").Error; err != nil {
			return nil, &SettlementError{Status: 500, Message: "Could not finish challenge"}
		}
	}

	// ── Finalise the market ─────────────────────────────────────────────────
	now := time.Now().UTC()
	market.ResolutionStatus = "Resolved"
	market.CorrectOption = correctOption
	market.ResolvedAt = &now
	market.ResolvedByID = adminID

	if err := tx.Save(&market).Error; err != nil {
		tx.Rollback()
		return nil, &SettlementError{Status: http.StatusInternalServerError, Message: "Failed to save resolved market"}
	}

	// Recalculate WinRate and TotalPredictions for all users involved in this market
	if len(predictions) > 0 {
		var userIDs []uint
		for _, p := range predictions {
			userIDs = append(userIDs, p.UserID)
		}

		if err := tx.Exec(`
			UPDATE users
			SET total_predictions = (
				SELECT COUNT(id) FROM prediction_submissions WHERE user_id = users.id AND deleted_at IS NULL
			),
			win_rate = COALESCE((
				SELECT (SUM(CASE WHEN is_correct = true THEN 1 ELSE 0 END) * 100.0) / NULLIF(COUNT(is_correct), 0)
				FROM prediction_submissions 
				WHERE user_id = users.id AND deleted_at IS NULL
			), 0)
			WHERE id IN ?
		`, userIDs).Error; err != nil {
			tx.Rollback()
			return nil, &SettlementError{Status: http.StatusInternalServerError, Message: "Failed to update user stats"}
		}
	}

	if err := LogAction(tx, adminID, "RESOLVE_MARKET", fmt.Sprintf("market_%d", market.ID), "Resolved market with winner: "+correctOption, input.ClientIP); err != nil {
		return nil, &SettlementError{Status: 500, Message: "Could not record settlement audit"}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, &SettlementError{Status: http.StatusInternalServerError, Message: "Transaction commit failed"}
	}
	BroadcastToAll("market_resolved", map[string]interface{}{"market_id": market.ID})
	BroadcastToAll("leaderboard_updated", map[string]interface{}{"market_id": market.ID})
	for _, p := range predictions {
		BroadcastToUser(p.UserID, "wallet_updated", map[string]interface{}{"market_id": market.ID})
		BroadcastToUser(p.UserID, "notification_created", map[string]interface{}{"market_id": market.ID})
	}

	return map[string]interface{}{
		"message":      "Market resolved successfully!",
		"market_id":    market.ID,
		"winner":       correctOption,
		"resolved_at":  now,
		"total_preds":  len(predictions),
		"winners_paid": winnerCount,
		"losers":       loserCount,
	}, nil
}
