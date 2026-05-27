package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/LYP-leo/xzxg-shop/backend/src/domain"
)

const (
	skillNavigateCart     = "navigate_cart"
	skillNavigateOrders   = "navigate_orders"
	skillNavigateProducts = "navigate_products"
	skillCouponHelp       = "coupon_help"
	skillOrderHelp        = "order_help"
	skillAfterSalesHelp   = "after_sales_help"
)

func (r *Runtime) executeSkill(ctx context.Context, run domain.AgentRun, call reactAction) toolObservation {
	startedAt := time.Now()
	skill := strings.TrimSpace(call.Skill)
	observation := toolObservation{Tool: skill, OK: false}
	defer func() {
		observation.DurationMS = time.Since(startedAt).Milliseconds()
	}()
	if skill == "" {
		observation.Message = "skill 名称为空"
		return observation
	}
	if r.store != nil && r.store.IsRunCanceled(ctx, run.RunID) {
		observation.Message = "运行已取消"
		return observation
	}
	switch skill {
	case skillNavigateCart:
		return skillAction(skill, "可以去购物车查看已选商品。", "打开购物车", "cart")
	case skillNavigateOrders:
		return skillAction(skill, "可以去订单页查看订单、支付和物流状态。", "查看订单", "orders")
	case skillNavigateProducts:
		return skillAction(skill, "可以去商品页浏览和筛选当前商品库。", "打开商品页", "products")
	case skillCouponHelp:
		return skillAction(skill, "优惠券入口在用户首页和结算相关区域；领取后可在可用优惠券列表查看门槛和有效期。", "查看优惠入口", "home")
	case skillOrderHelp:
		return skillAction(skill, "订单页可以查看待支付、待收货、已完成订单；物流和售后入口也从订单详情进入。", "查看订单", "orders")
	case skillAfterSalesHelp:
		return skillAction(skill, "售后需要先定位对应订单和商品，再在订单详情里发起退换货或联系商家处理。", "去订单页处理售后", "orders")
	default:
		observation.Message = "未知 skill：" + skill
		return observation
	}
}

func skillAction(skill string, message string, label string, target string) toolObservation {
	block := domain.AgentBlock{
		Type:    "action",
		Message: message,
		Action: map[string]interface{}{
			"name":   "navigate",
			"target": target,
			"label":  label,
		},
	}
	return toolObservation{
		Tool:    skill,
		OK:      true,
		Message: message,
		Result: map[string]any{
			"skill":   skill,
			"action":  block.Action,
			"message": message,
		},
		Blocks: []domain.AgentBlock{block},
	}
}

func skillObservationForTrace(observation toolObservation) map[string]any {
	raw, _ := json.Marshal(observation.Blocks)
	return map[string]any{
		"ok":               observation.OK,
		"message":          observation.Message,
		"result":           observation.Result,
		"blocks":           observation.Blocks,
		"blocks_json":      string(raw),
		"duration_ms":      observation.DurationMS,
		"observation_json": observationForModel(observation),
	}
}

func statusTextForSkill(skill string) string {
	switch skill {
	case skillNavigateCart:
		return "正在准备购物车入口"
	case skillNavigateProducts:
		return "正在准备商品页入口"
	case skillNavigateOrders, skillOrderHelp, skillAfterSalesHelp:
		return "正在准备订单入口"
	case skillCouponHelp:
		return "正在准备优惠入口"
	default:
		return fmt.Sprintf("正在执行 %s", skill)
	}
}
