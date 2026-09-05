package httpapi

import (
	"context"
	"net/http"

	"github.com/LYP-leo/xzxg-shop/backend/src/store"
)

func (s *Server) handleInventoryAudit(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	auditor, ok := s.store.(interface {
		InventoryAudit(context.Context) ([]store.InventoryAuditIssue, error)
	})
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "audit_unavailable", "库存审计暂不可用")
		return
	}
	issues, err := auditor.InventoryAudit(r.Context())
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "audit_failed", "库存审计失败，请稍后重试")
		return
	}
	truncated := len(issues) > 1000
	if truncated {
		issues = issues[:1000]
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": issues, "truncated": truncated})
}
