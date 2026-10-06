package agent

import (
	"context"
	"encoding/json"
	"fahd-backend/internal/ai"
	"fahd-backend/internal/ai/services"
	"fmt"
	"strings"
)

func (s *Service) directResponse(ctx context.Context, r MessageRequest, id int64) (MessageResponse, bool) {
	text := services.NormalizeSearch(strings.Trim(r.Message, " ؟?!,."))
	kind := ""
	switch text {
	case "العروض", "ابي اعرف العروض", "ابي العروض", "وريني العروض", "offers":
		kind = "show_offers"
	case "اطلب الان", "ابي اطلب", "طلب المنتج":
		kind = "checkout"
	case "السعر", "بكم", "كم السعر", "ابي اعرف السعر":
		kind = "price"
	case "المخزون", "كم المتوفر":
		kind = "stock"
	}
	if kind == "" {
		return MessageResponse{}, false
	}
	raw, ok := r.Context["productId"]
	if !ok {
		return MessageResponse{}, false
	}
	var productID int64
	switch v := raw.(type) {
	case float64:
		productID = int64(v)
	case int64:
		productID = v
	case int:
		productID = int64(v)
	}
	if productID < 1 {
		return MessageResponse{}, false
	}
	var p struct {
		Title       string
		Price       float64
		StockTotal  int
		HasVariants bool
	}
	if e := s.db.WithContext(ctx).Table("products").Where("id=? AND status='active'", productID).Take(&p).Error; e != nil {
		return MessageResponse{}, false
	}
	reply := "هذه العروض المتاحة للمنتج. اختر الأنسب لك، والشحن يظهر حسب مدينتك قبل إرسال الطلب."
	actions := []any{AgentAction{Type: kind}}
	if kind == "checkout" {
		reply = "راجع العرض وبيانات التوصيل في نموذج الطلب. الدفع عند الاستلام."
	}
	if kind == "price" {
		q, e := services.NewOrderService(s.db).CalculateTotal(ctx, services.CreateOrderInput{SessionID: r.SessionID, Items: []services.OrderItemInput{{ProductID: productID, Qty: 1}}})
		if e != nil {
			reply = e.Error()
		} else {
			reply = fmt.Sprintf("سعر قطعة من %s: %.2f ر.س. الشحن حسب المدينة ويظهر قبل إرسال الطلب.", p.Title, q["grandTotal"])
		}
		actions = []any{AgentAction{Type: "show_offers"}}
	}
	if kind == "stock" {
		reply = fmt.Sprintf("المتوفر الآن من %s: %d قطعة.", p.Title, p.StockTotal)
		if p.HasVariants {
			reply = "اختر المقاس أو اللون لعرض الكمية المتاحة لهذا الخيار."
		}
		actions = []any{}
	}
	if kind != "checkout" {
		actions = append(actions, AgentAction{Type: "quick_reply", Payload: map[string]any{"label": "التوصيل والشحن", "message": "كم تكلفة الشحن وموعد التوصيل لهذا المنتج؟"}})
		if kind == "stock" {
			actions = append(actions, AgentAction{Type: "quick_reply", Payload: map[string]any{"label": "أظهر العروض", "message": "العروض"}})
		} else {
			actions = append(actions, AgentAction{Type: "quick_reply", Payload: map[string]any{"label": "مميزات المنتج", "message": "وش أهم مميزات هذا المنتج ولمن يناسب؟"}})
		}
	}
	encoded, _ := json.Marshal(actions)
	_ = s.conversations.SaveMessage(ctx, id, ai.RoleAssistant, reply, map[string]any{"actions": json.RawMessage(encoded)})
	if r.OnText != nil {
		r.OnText(reply)
	}
	return MessageResponse{ConversationID: id, Reply: reply, Actions: actions, Meta: defaultMeta(false, nil)}, true
}
