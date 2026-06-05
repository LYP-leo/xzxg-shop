package agent

import (
	"context"
	"strings"
)

const (
	modelRolePlannerRoute       = "planner_route"
	modelRolePlannerGuideIntent = "planner_guide_intent"
	modelRolePlannerNonGuide    = "planner_non_guide_intent"
	modelRoleMemoryRetrieval    = "memory_retrieval"
	modelRoleMemorySummary      = "memory_summary"
	modelRoleFollowups          = "followups"
	modelRoleProductFilter      = "product_filter"
	modelRoleReactGuide         = "react_guide"
	modelRoleReactNonGuide      = "react_non_guide"
	modelRoleReactToolIntent    = "react_tool_intent"
)

func (r *Runtime) modelForRole(ctx context.Context, role string, fallback string) string {
	role = strings.TrimSpace(role)
	if role == "" {
		return fallback
	}
	if r.configs == nil {
		return fallback
	}
	values := r.configs.GetMap(ctx)
	for _, key := range []string{
		"ai.model." + role,
		"agent.model." + role,
	} {
		if value := strings.TrimSpace(values[key]); value != "" {
			return value
		}
	}
	return fallback
}

func (r *Runtime) answerModelForPlan(ctx context.Context, plan runPlan) string {
	if isToolIntent(plan.Intent) {
		return r.modelForRole(ctx, modelRoleReactToolIntent, r.llm.SmallModel())
	}
	if plan.Route == "non_guide" {
		return r.modelForRole(ctx, modelRoleReactNonGuide, r.llm.SmallModel())
	}
	return r.modelForRole(ctx, modelRoleReactGuide, r.llm.LargeModel())
}
