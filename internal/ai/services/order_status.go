package services

import (
	"context"
	"encoding/json"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

func (s *OrderService) ChangeStatus(ctx context.Context, id int64, status string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var o struct {
			ID                           int64
			Status, SessionID, VisitorID string
			StockReserved                bool
		}
		if e := tx.Table("orders").Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=?", id).Take(&o).Error; e != nil {
			return e
		}
		if o.Status == status {
			return nil
		}
		allowed := map[string][]string{"new": {"confirmed", "cancelled"}, "confirmed": {"processing", "shipped", "cancelled"}, "processing": {"shipped", "cancelled"}, "shipped": {"delivered", "returned"}, "delivered": {"returned"}}
		valid := false
		for _, v := range allowed[o.Status] {
			if v == status {
				valid = true
			}
		}
		if !valid {
			return fmt.Errorf("انتقال حالة الطلب غير مسموح")
		}
		reserved := o.StockReserved
		// Only unshipped cancellations are automatically restocked. Returned goods need inspection.
		if status == "cancelled" && reserved {
			var items []struct {
				ProductID int64
				VariantID *int64
				Qty       int
			}
			if e := tx.Table("order_items").Where("order_id=?", id).Order("product_id,variant_id").Find(&items).Error; e != nil {
				return e
			}
			for _, it := range items {
				var mode string
				if e := tx.Table("products").Select("inventory_mode").Where("id=?", it.ProductID).Scan(&mode).Error; e != nil {
					return e
				}
				if mode == "variant" && it.VariantID != nil {
					if e := tx.Table("product_variants").Where("id=?", *it.VariantID).Update("stock", gorm.Expr("stock+?", it.Qty)).Error; e != nil {
						return e
					}
				} else {
					if e := tx.Table("products").Where("id=?", it.ProductID).Update("stock_total", gorm.Expr("stock_total+?", it.Qty)).Error; e != nil {
						return e
					}
				}
			}
			reserved = false
		}
		if e := tx.Table("orders").Where("id=?", id).Updates(map[string]any{"status": status, "stock_reserved": reserved, "updated_at": time.Now()}).Error; e != nil {
			return e
		}
		if status == "confirmed" || status == "delivered" || status == "cancelled" || status == "returned" {
			meta, _ := json.Marshal(map[string]any{"orderId": id})
			return tx.Table("analytics_events").Create(map[string]any{"event_type": "order_" + status, "session_id": o.SessionID, "visitor_id": o.VisitorID, "metadata": string(meta)}).Error
		}
		return nil
	})
}
