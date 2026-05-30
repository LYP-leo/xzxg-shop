package agent

import (
	"context"
	"testing"

	"github.com/LYP-leo/xzxg-shop/backend/src/domain"
)

func TestExecuteSkillNavigateCart(t *testing.T) {
	runtime := &Runtime{}
	observation := runtime.executeSkill(context.Background(), domain.AgentRun{RunID: "run_1"}, reactAction{
		Type:  skillNavigateCart,
		Skill: skillNavigateCart,
	})
	if !observation.OK {
		t.Fatalf("observation should be ok: %s", observation.Message)
	}
	if len(observation.Blocks) != 1 {
		t.Fatalf("blocks len = %d, want 1", len(observation.Blocks))
	}
	action := observation.Blocks[0].Action
	if action["name"] != "navigate" || action["target"] != "cart" {
		t.Fatalf("action = %#v, want navigate cart", action)
	}
}

func TestExecuteSkillNavigateProducts(t *testing.T) {
	runtime := &Runtime{}
	observation := runtime.executeSkill(context.Background(), domain.AgentRun{RunID: "run_1"}, reactAction{
		Type:  "skill_call",
		Skill: skillNavigateProducts,
	})
	if !observation.OK {
		t.Fatalf("observation should be ok: %s", observation.Message)
	}
	if got := observation.Blocks[0].Action["target"]; got != "products" {
		t.Fatalf("target = %v, want products", got)
	}
}
