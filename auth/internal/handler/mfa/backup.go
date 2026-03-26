package mfa

import (
	"net/http"

	"go.uber.org/zap"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/LegationPro/zagforge/auth/internal/handler"
	mfasvc "github.com/LegationPro/zagforge/auth/internal/service/mfa"
	authstore "github.com/LegationPro/zagforge/auth/internal/store"
	"github.com/LegationPro/zagforge/shared/go/httputil"
)

// RegenerateBackupCodes generates a fresh set of backup codes, replacing any existing ones.
func (h *Handler) RegenerateBackupCodes(w http.ResponseWriter, r *http.Request) {
	userID, _, err := userFromContext(r)
	if err != nil {
		httputil.ErrResponse(w, http.StatusUnauthorized, err)
		return
	}

	settings, err := h.db.Queries.GetMFASettings(r.Context(), userID)
	if err != nil || !settings.TotpEnabled {
		httputil.ErrResponse(w, http.StatusBadRequest, errMFANotEnabled)
		return
	}

	codes, err := generateAndStoreBackupCodes(r, h.db.Queries, userID)
	if err != nil {
		h.log.Error("regenerate backup codes", zap.Error(err))
		httputil.ErrResponse(w, http.StatusInternalServerError, handler.ErrInternal)
		return
	}

	httputil.OkResponse(w, backupCodesResponse{BackupCodes: codes})
}

// generateAndStoreBackupCodes deletes old codes, generates new ones, stores hashes, returns plaintexts.
func generateAndStoreBackupCodes(r *http.Request, queries *authstore.Queries, userID pgtype.UUID) ([]string, error) {
	_ = queries.DeleteBackupCodes(r.Context(), userID)

	codes, err := mfasvc.GenerateBackupCodes()
	if err != nil {
		return nil, err
	}

	params := make([]authstore.CreateBackupCodesParams, len(codes))
	for i, c := range codes {
		params[i] = authstore.CreateBackupCodesParams{
			UserID:   userID,
			CodeHash: c.Hash,
		}
	}

	if _, err := queries.CreateBackupCodes(r.Context(), params); err != nil {
		return nil, err
	}

	plaintexts := make([]string, len(codes))
	for i, c := range codes {
		plaintexts[i] = c.Plain
	}
	return plaintexts, nil
}
