package constants

// Статусы заявок
const (
	StatusAwaitPay       = "await_pay"
	StatusReceiptPending = "receipt_pending"
	StatusPaid           = "paid"
	StatusRejected       = "rejected"
	StatusDone           = "done"
)

// Человекочитаемые названия статусов
func HumanStatus(status string) string {
	switch status {
	case StatusAwaitPay:
		return "ожидает оплату"
	case StatusReceiptPending:
		return "чек на модерации"
	case StatusPaid:
		return "оплачен"
	case StatusRejected:
		return "чек отклонён"
	case StatusDone:
		return "завершено"
	default:
		return status
	}
}
