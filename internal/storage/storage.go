package storage

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"tgbot/internal/constants"
	"time"

	_ "modernc.org/sqlite"
)

type Order struct {
	ID                          int64
	UserID                      int64
	ChatID                      int64
	CreatedAt                   int64
	Service, DeadlineRaw, Pages string
	Topic                       string // ← НОВОЕ: тема работы
	Requirements                string // ← НОВОЕ: требования
	ClientSource                string
	Status                      string
	LastReceiptID               string
	LastReceiptType             string
	PaymentAmount               float64 // сумма платежа
	PaymentDate                 int64   // дата оплаты

	// Новые поля для "канбана" (карточка в forum topic)
	CurrentBoardMsgID  int64 // message_id карточки в борде (последняя актуальная)
	CurrentBoardThread int   // message_thread_id темы, где сейчас лежит карточка
}

type Store struct{ DB *sql.DB }

const (
	CurrentSchemaVersion = 6
)

func Open(path string) (*Store, error) {
	// Можно оставить как было; этот формат тоже валиден
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(ON)")
	if err != nil {
		return nil, err
	}

	// Инициализация схемы
	if err := initSchema(db); err != nil {
		return nil, err
	}

	// Выполнение миграций
	if err := runMigrations(db); err != nil {
		return nil, err
	}

	return &Store{DB: db}, nil
}

func initSchema(db *sql.DB) error {
	// Базовая схема (для НОВОЙ базы сразу с новыми колонками)
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS orders (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL,
  chat_id INTEGER NOT NULL,
  created_at INTEGER NOT NULL,
  service TEXT NOT NULL,
  deadline_raw TEXT,
  pages TEXT,
  topic TEXT,
  requirements TEXT,
  client_source TEXT,
  status TEXT NOT NULL,
  last_receipt_id TEXT,
  last_receipt_type TEXT,
  payment_amount REAL DEFAULT 0,
  payment_date INTEGER DEFAULT 0,
  current_board_msg_id INTEGER DEFAULT 0,
  current_board_thread INTEGER DEFAULT 0
)`)
	if err != nil {
		return err
	}

	// Создаем индексы
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_orders_user ON orders(user_id)`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_orders_status ON orders(status)`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_orders_payment_date ON orders(payment_date)`)
	if err != nil {
		return err
	}

	return nil
}

// GetUsersCount возвращает общее количество пользователей
func (s *Store) GetUsersCount() (int64, error) {
	var count int64
	err := s.DB.QueryRow("SELECT COUNT(DISTINCT user_id) FROM orders").Scan(&count)
	return count, err
}

// GetOrdersCount возвращает общее количество заявок
func (s *Store) GetOrdersCount() (int64, error) {
	var count int64
	err := s.DB.QueryRow("SELECT COUNT(*) FROM orders").Scan(&count)
	return count, err
}

// GetOrdersCountSince возвращает количество заявок с указанной даты
func (s *Store) GetOrdersCountSince(since time.Time) (int64, error) {
	var count int64
	err := s.DB.QueryRow("SELECT COUNT(*) FROM orders WHERE created_at >= ?", since.Unix()).Scan(&count)
	return count, err
}

// GetOrdersByStatusSince возвращает заявки по статусу с указанной даты
func (s *Store) GetOrdersByStatusSince(status string, since time.Time) ([]Order, error) {
	rows, err := s.DB.Query(`
		SELECT id, user_id, chat_id, created_at, service, deadline_raw, pages, topic, requirements, status, 
		       last_receipt_id, last_receipt_type, payment_amount, payment_date,
		       current_board_msg_id, current_board_thread
		FROM orders 
		WHERE status = ? AND created_at >= ?
		ORDER BY created_at DESC
	`, status, since.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []Order
	for rows.Next() {
		var o Order
		err := rows.Scan(&o.ID, &o.UserID, &o.ChatID, &o.CreatedAt, &o.Service,
			&o.DeadlineRaw, &o.Pages, &o.Topic, &o.Requirements, &o.Status, &o.LastReceiptID,
			&o.LastReceiptType, &o.PaymentAmount, &o.PaymentDate,
			&o.CurrentBoardMsgID, &o.CurrentBoardThread)
		if err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}
	return orders, nil
}

// GetWorkTypeStats возвращает статистику по типам работ за период
func (s *Store) GetWorkTypeStats(since time.Time) (map[string]int64, error) {
	rows, err := s.DB.Query(`
		SELECT service, COUNT(*) as count
		FROM orders 
		WHERE created_at >= ?
		GROUP BY service
		ORDER BY count DESC
	`, since.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := make(map[string]int64)
	for rows.Next() {
		var workType string
		var count int64
		if err := rows.Scan(&workType, &count); err != nil {
			return nil, err
		}
		stats[workType] = count
	}
	return stats, nil
}

// GetInstitutionStats возвращает статистику по заведениям за период
func (s *Store) GetInstitutionStats(since time.Time) (map[string]int64, error) {
	rows, err := s.DB.Query(`
		SELECT 
			CASE 
				WHEN service LIKE '%Вуз%' OR service LIKE '%Университет%' THEN 'Вуз'
				WHEN service LIKE '%Колледж%' THEN 'Колледж'
				ELSE 'Другое'
			END as institution,
			COUNT(*) as count
		FROM orders 
		WHERE created_at >= ?
		GROUP BY institution
		ORDER BY count DESC
	`, since.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := make(map[string]int64)
	for rows.Next() {
		var institution string
		var count int64
		if err := rows.Scan(&institution, &count); err != nil {
			return nil, err
		}
		stats[institution] = count
	}
	return stats, nil
}

// GetTopOrdersByPages возвращает топ заявки по объему страниц за период
func (s *Store) GetTopOrdersByPages(since time.Time, limit int) ([]Order, error) {
	rows, err := s.DB.Query(`
		SELECT id, user_id, chat_id, created_at, service, deadline_raw, pages, topic, requirements, status, 
		       last_receipt_id, last_receipt_type, payment_amount, payment_date,
		       current_board_msg_id, current_board_thread
		FROM orders 
		WHERE created_at >= ?
		ORDER BY CAST(pages AS INTEGER) DESC
		LIMIT ?
	`, since.Unix(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []Order
	for rows.Next() {
		var o Order
		err := rows.Scan(&o.ID, &o.UserID, &o.ChatID, &o.CreatedAt, &o.Service,
			&o.DeadlineRaw, &o.Pages, &o.Topic, &o.Requirements, &o.Status, &o.LastReceiptID,
			&o.LastReceiptType, &o.PaymentAmount, &o.PaymentDate,
			&o.CurrentBoardMsgID, &o.CurrentBoardThread)
		if err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}
	return orders, nil
}

func runMigrations(db *sql.DB) error {
	var currentVersion int
	err := db.QueryRow(`PRAGMA user_version`).Scan(&currentVersion)
	if err != nil {
		return err
	}

	// Если версия уже актуальная, миграции не нужны
	if currentVersion >= CurrentSchemaVersion {
		return nil
	}

	// Миграция с версии 0 до 1: добавление колонок для доски
	if currentVersion < 1 {
		if _, err = db.Exec(`ALTER TABLE orders ADD COLUMN current_board_msg_id INTEGER DEFAULT 0;`); err != nil {
			if !isDupColumnErr(err) {
				return err
			}
		}
		if _, err = db.Exec(`ALTER TABLE orders ADD COLUMN current_board_thread INTEGER DEFAULT 0;`); err != nil {
			if !isDupColumnErr(err) {
				return err
			}
		}
	}

	// Миграция с версии 1 до 2: добавление индексов
	if currentVersion < 2 {
		_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_orders_status ON orders(status);`)
		if err != nil {
			return err
		}
	}

	// Миграция с версии 2 до 3: добавление полей для платежей
	if currentVersion < 3 {
		if _, err = db.Exec(`ALTER TABLE orders ADD COLUMN payment_amount REAL DEFAULT 0;`); err != nil {
			if !isDupColumnErr(err) {
				return err
			}
		}
		if _, err = db.Exec(`ALTER TABLE orders ADD COLUMN payment_date INTEGER DEFAULT 0;`); err != nil {
			if !isDupColumnErr(err) {
				return err
			}
		}
		_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_orders_payment_date ON orders(payment_date);`)
		if err != nil {
			return err
		}
	}

	// Миграция с версии 3 до 4: удаление поля faculty
	if currentVersion < 4 {
		// SQLite не поддерживает DROP COLUMN напрямую, поэтому создаем новую таблицу
		_, err = db.Exec(`
			CREATE TABLE orders_new (
			  id INTEGER PRIMARY KEY AUTOINCREMENT,
			  user_id INTEGER NOT NULL,
			  chat_id INTEGER NOT NULL,
			  created_at INTEGER NOT NULL,
			  service TEXT NOT NULL,
			  deadline_raw TEXT,
			  pages TEXT,
			  notes TEXT,
			  status TEXT NOT NULL,
			  last_receipt_id TEXT,
			  last_receipt_type TEXT,
			  payment_amount REAL DEFAULT 0,
			  payment_date INTEGER DEFAULT 0,
			  current_board_msg_id INTEGER DEFAULT 0,
			  current_board_thread INTEGER DEFAULT 0
			)`)
		if err != nil {
			return err
		}

		// Копируем данные без faculty
		_, err = db.Exec(`
			INSERT INTO orders_new 
			(id, user_id, chat_id, created_at, service, deadline_raw, pages, notes, status, 
			 last_receipt_id, last_receipt_type, payment_amount, payment_date, 
			 current_board_msg_id, current_board_thread)
			SELECT id, user_id, chat_id, created_at, service, deadline_raw, pages, notes, status,
			       last_receipt_id, last_receipt_type, payment_amount, payment_date,
			       current_board_msg_id, current_board_thread
			FROM orders`)
		if err != nil {
			return err
		}

		// Удаляем старую таблицу и переименовываем новую
		_, err = db.Exec(`DROP TABLE orders`)
		if err != nil {
			return err
		}

		_, err = db.Exec(`ALTER TABLE orders_new RENAME TO orders`)
		if err != nil {
			return err
		}

		// Восстанавливаем индексы
		_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_orders_user ON orders(user_id)`)
		if err != nil {
			return err
		}
		_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_orders_status ON orders(status)`)
		if err != nil {
			return err
		}
		_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_orders_payment_date ON orders(payment_date)`)
		if err != nil {
			return err
		}
	}

	// Миграция с версии 4 до 5: разделение notes на topic и requirements
	if currentVersion < 5 {
		// Добавляем новые колонки
		_, err = db.Exec(`ALTER TABLE orders ADD COLUMN topic TEXT`)
		if err != nil && !isDupColumnErr(err) {
			return err
		}
		_, err = db.Exec(`ALTER TABLE orders ADD COLUMN requirements TEXT`)
		if err != nil && !isDupColumnErr(err) {
			return err
		}

		// Заполняем новые поля значениями по умолчанию для существующих записей
		_, err = db.Exec(`UPDATE orders SET topic = 'Не указано', requirements = 'Не указано' WHERE topic IS NULL OR requirements IS NULL`)
		if err != nil {
			return err
		}
	}

	if currentVersion < 6 {
		if _, err = db.Exec(`ALTER TABLE orders ADD COLUMN client_source TEXT`); err != nil && !isDupColumnErr(err) {
			return err
		}
	}

	_, err = db.Exec(fmt.Sprintf("PRAGMA user_version = %d", CurrentSchemaVersion))
	return err
}

// Вспомогательная: ошибка "duplicate column name"
func isDupColumnErr(err error) bool {
	if err == nil {
		return false
	}
	// В modernc/sqlite текст ошибки обычно содержит это:
	msg := err.Error()
	return strings.Contains(strings.ToLower(msg), "duplicate column name")
}

// scanOrderWithNulls сканирует заказ с обработкой NULL значений для topic и requirements
func scanOrderWithNulls(rows *sql.Rows, o *Order) error {
	var topicNull, requirementsNull sql.NullString

	var clientSourceNull sql.NullString

	err := rows.Scan(&o.ID, &o.UserID, &o.ChatID, &o.CreatedAt, &o.Service, &o.DeadlineRaw, &o.Pages,
		&topicNull, &requirementsNull, &clientSourceNull, &o.Status, &o.LastReceiptID, &o.LastReceiptType,
		&o.PaymentAmount, &o.PaymentDate, &o.CurrentBoardMsgID, &o.CurrentBoardThread)

	if err != nil {
		return err
	}

	// Обрабатываем NULL значения
	if topicNull.Valid {
		o.Topic = topicNull.String
	} else {
		o.Topic = ""
	}

	if requirementsNull.Valid {
		o.Requirements = requirementsNull.String
	} else {
		o.Requirements = ""
	}

	if clientSourceNull.Valid {
		o.ClientSource = clientSourceNull.String
	}

	return nil
}

func (s *Store) CreateOrder(o *Order) (int64, error) {
	o.CreatedAt = time.Now().Unix()
	res, err := s.DB.Exec(`INSERT INTO orders
(user_id,chat_id,created_at,service,deadline_raw,pages,topic,requirements,client_source,status,last_receipt_id,last_receipt_type,payment_amount,payment_date,current_board_msg_id,current_board_thread)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		o.UserID, o.ChatID, o.CreatedAt, o.Service, o.DeadlineRaw, o.Pages, o.Topic, o.Requirements, o.ClientSource, o.Status, o.LastReceiptID, o.LastReceiptType, o.PaymentAmount, o.PaymentDate, o.CurrentBoardMsgID, o.CurrentBoardThread,
	)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	o.ID = id
	return id, nil
}

func (s *Store) GetUserOrdersPaginated(uid int64, page, pageSize int) ([]Order, int, error) {
	offset := (page - 1) * pageSize

	rows, err := s.DB.Query(
		`SELECT id, service, deadline_raw, status, pages
		   FROM orders
		  WHERE user_id = ?
		  ORDER BY id DESC
		  LIMIT ? OFFSET ?`,
		uid, pageSize, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var orders []Order
	for rows.Next() {
		var o Order
		if err := rows.Scan(&o.ID, &o.Service, &o.DeadlineRaw, &o.Status, &o.Pages); err != nil {
			return nil, 0, err
		}
		orders = append(orders, o)
	}

	// Общее количество заявок пользователя
	var total int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM orders WHERE user_id = ?`, uid).Scan(&total); err != nil {
		return nil, 0, err
	}

	return orders, total, nil
}

func (s *Store) LoadOrder(id int64) (*Order, error) {
	o := &Order{}
	row := s.DB.QueryRow(`SELECT
  id,user_id,chat_id,created_at,service,deadline_raw,pages,topic,requirements,client_source,status,last_receipt_id,last_receipt_type,
  payment_amount,payment_date,current_board_msg_id,current_board_thread
FROM orders WHERE id=?`, id)

	var topicNull, requirementsNull, clientSourceNull sql.NullString
	err := row.Scan(&o.ID, &o.UserID, &o.ChatID, &o.CreatedAt, &o.Service, &o.DeadlineRaw, &o.Pages,
		&topicNull, &requirementsNull, &clientSourceNull, &o.Status, &o.LastReceiptID, &o.LastReceiptType,
		&o.PaymentAmount, &o.PaymentDate, &o.CurrentBoardMsgID, &o.CurrentBoardThread)

	if err != nil {
		return nil, err
	}

	// Обрабатываем NULL значения
	if topicNull.Valid {
		o.Topic = topicNull.String
	} else {
		o.Topic = ""
	}

	if requirementsNull.Valid {
		o.Requirements = requirementsNull.String
	} else {
		o.Requirements = ""
	}

	if clientSourceNull.Valid {
		o.ClientSource = clientSourceNull.String
	}

	return o, nil
}

func (s *Store) SetStatus(id int64, status string) error {
	_, err := s.DB.Exec(`UPDATE orders SET status=? WHERE id=?`, status, id)
	return err
}

// SetPaymentAmount устанавливает сумму платежа и дату оплаты
func (s *Store) SetPaymentAmount(id int64, amount float64) error {
	_, err := s.DB.Exec(`UPDATE orders SET payment_amount=?, payment_date=? WHERE id=?`, amount, time.Now().Unix(), id)
	return err
}

// GetPaymentAmount возвращает сумму платежа для заявки
func (s *Store) GetPaymentAmount(id int64) (float64, error) {
	var amount float64
	err := s.DB.QueryRow(`SELECT payment_amount FROM orders WHERE id=?`, id).Scan(&amount)
	return amount, err
}

func (s *Store) SaveReceipt(id int64, fileID, typ string) error {
	_, err := s.DB.Exec(`UPDATE orders SET last_receipt_id=?, last_receipt_type=?, status=? WHERE id=?`, fileID, typ, constants.StatusReceiptPending, id)
	return err
}

// WeeklyStats содержит статистику за неделю
type WeeklyStats struct {
	TotalOrders    int     // общее количество заявок
	TotalRevenue   float64 // общая выручка
	TotalRefunds   float64 // общая сумма возвратов
	RejectedOrders int     // количество отклоненных заявок
	PaidOrders     int     // количество оплаченных заявок
	PendingOrders  int     // количество заявок в ожидании
}

// GetWeeklyStats возвращает статистику за указанную неделю
func (s *Store) GetWeeklyStats(startDate, endDate int64) (*WeeklyStats, error) {
	stats := &WeeklyStats{}

	// Общее количество заявок за период
	err := s.DB.QueryRow(`SELECT COUNT(*) FROM orders WHERE created_at BETWEEN ? AND ?`, startDate, endDate).Scan(&stats.TotalOrders)
	if err != nil {
		return nil, err
	}

	// Общая выручка (сумма всех оплаченных заявок)
	err = s.DB.QueryRow(`SELECT COALESCE(SUM(payment_amount), 0)
  FROM orders
 WHERE status IN (?, ?) AND payment_date BETWEEN ? AND ?`,
		constants.StatusPaid, constants.StatusDone, startDate, endDate).Scan(&stats.TotalRevenue)
	if err != nil {
		return nil, err
	}

	// Количество отклоненных заявок
	err = s.DB.QueryRow(`SELECT COUNT(*) FROM orders WHERE status = ? AND created_at BETWEEN ? AND ?`, constants.StatusRejected, startDate, endDate).Scan(&stats.RejectedOrders)
	if err != nil {
		return nil, err
	}

	// Количество оплаченных заявок
	err = s.DB.QueryRow(`SELECT COUNT(*) FROM orders WHERE status = ? AND payment_date BETWEEN ? AND ?`, constants.StatusPaid, startDate, endDate).Scan(&stats.PaidOrders)
	if err != nil {
		return nil, err
	}

	// Количество заявок в ожидании оплаты
	err = s.DB.QueryRow(`SELECT COUNT(*) FROM orders WHERE status = ? AND created_at BETWEEN ? AND ?`, constants.StatusAwaitPay, startDate, endDate).Scan(&stats.PendingOrders)
	if err != nil {
		return nil, err
	}

	// TODO: Добавить логику для возвратов, когда будет реализована система возвратов
	stats.TotalRefunds = 0

	return stats, nil
}

// GetMonthlyStats возвращает статистику за указанный месяц
func (s *Store) GetMonthlyStats(startDate, endDate int64) (*WeeklyStats, error) {
	stats := &WeeklyStats{}

	// Общее количество заявок за период
	err := s.DB.QueryRow(`SELECT COUNT(*) FROM orders WHERE created_at BETWEEN ? AND ?`, startDate, endDate).Scan(&stats.TotalOrders)
	if err != nil {
		return nil, err
	}

	// Общая выручка (сумма всех оплаченных заявок)
	err = s.DB.QueryRow(`SELECT COALESCE(SUM(payment_amount), 0) FROM orders WHERE status = ? AND payment_date BETWEEN ? AND ?`, constants.StatusPaid, startDate, endDate).Scan(&stats.TotalRevenue)
	if err != nil {
		return nil, err
	}

	// Количество отклоненных заявок
	err = s.DB.QueryRow(`SELECT COUNT(*) FROM orders WHERE status = ? AND created_at BETWEEN ? AND ?`, constants.StatusRejected, startDate, endDate).Scan(&stats.RejectedOrders)
	if err != nil {
		return nil, err
	}

	// Количество оплаченных заявок
	err = s.DB.QueryRow(`SELECT COUNT(*)
  FROM orders
 WHERE status IN (?, ?) AND payment_date BETWEEN ? AND ?`,
		constants.StatusPaid, constants.StatusDone, startDate, endDate).Scan(&stats.PaidOrders)
	if err != nil {
		return nil, err
	}

	// Количество заявок в ожидании оплаты
	err = s.DB.QueryRow(`SELECT COUNT(*) FROM orders WHERE status = ? AND created_at BETWEEN ? AND ?`, constants.StatusAwaitPay, startDate, endDate).Scan(&stats.PendingOrders)
	if err != nil {
		return nil, err
	}

	stats.TotalRefunds = 0

	return stats, nil
}

// GetYearlyStats возвращает статистику за указанный год
func (s *Store) GetYearlyStats(startDate, endDate int64) (*WeeklyStats, error) {
	stats := &WeeklyStats{}

	// Общее количество заявок за период
	err := s.DB.QueryRow(`SELECT COUNT(*) FROM orders WHERE created_at BETWEEN ? AND ?`, startDate, endDate).Scan(&stats.TotalOrders)
	if err != nil {
		return nil, err
	}

	// Общая выручка (сумма всех оплаченных заявок)
	err = s.DB.QueryRow(`SELECT COALESCE(SUM(payment_amount), 0) FROM orders WHERE status = ? AND payment_date BETWEEN ? AND ?`, constants.StatusPaid, startDate, endDate).Scan(&stats.TotalRevenue)
	if err != nil {
		return nil, err
	}

	// Количество отклоненных заявок
	err = s.DB.QueryRow(`SELECT COUNT(*) FROM orders WHERE status = ? AND created_at BETWEEN ? AND ?`, constants.StatusRejected, startDate, endDate).Scan(&stats.RejectedOrders)
	if err != nil {
		return nil, err
	}

	// Количество оплаченных заявок
	err = s.DB.QueryRow(`SELECT COUNT(*) FROM orders WHERE status = ? AND payment_date BETWEEN ? AND ?`, constants.StatusPaid, startDate, endDate).Scan(&stats.PaidOrders)
	if err != nil {
		return nil, err
	}

	// Количество заявок в ожидании оплаты
	err = s.DB.QueryRow(`SELECT COUNT(*) FROM orders WHERE status = ? AND created_at BETWEEN ? AND ?`, constants.StatusAwaitPay, startDate, endDate).Scan(&stats.PendingOrders)
	if err != nil {
		return nil, err
	}

	stats.TotalRefunds = 0

	return stats, nil
}

// GetTotalStats возвращает общую статистику за все время
func (s *Store) GetTotalStats() (*WeeklyStats, error) {
	stats := &WeeklyStats{}

	// Общее количество заявок
	err := s.DB.QueryRow(`SELECT COUNT(*) FROM orders`).Scan(&stats.TotalOrders)
	if err != nil {
		return nil, err
	}

	// Общая выручка (сумма всех оплаченных заявок)
	err = s.DB.QueryRow(`SELECT COALESCE(SUM(payment_amount), 0)
  FROM orders WHERE status IN (?, ?)`,
		constants.StatusPaid, constants.StatusDone).Scan(&stats.TotalRevenue)
	if err != nil {
		return nil, err
	}

	// Количество отклоненных заявок
	err = s.DB.QueryRow(`SELECT COUNT(*) FROM orders WHERE status = ?`, constants.StatusRejected).Scan(&stats.RejectedOrders)
	if err != nil {
		return nil, err
	}

	// Количество оплаченных заявок
	err = s.DB.QueryRow(`SELECT COUNT(*)
  FROM orders WHERE status IN (?, ?)`,
		constants.StatusPaid, constants.StatusDone).Scan(&stats.PaidOrders)
	if err != nil {
		return nil, err
	}

	// Количество заявок в ожидании оплаты
	err = s.DB.QueryRow(`SELECT COUNT(*) FROM orders WHERE status = ?`, constants.StatusAwaitPay).Scan(&stats.PendingOrders)
	if err != nil {
		return nil, err
	}

	stats.TotalRefunds = 0

	return stats, nil
}

// ClearAllOrders очищает все заявки (только для тестирования!)
func (s *Store) ClearAllOrders() error {
	_, err := s.DB.Exec(`DELETE FROM orders`)
	return err
}

// GetOrdersByDeadline возвращает заявки с приближающимися дедлайнами
func (s *Store) GetOrdersByDeadline(daysAhead int) ([]Order, error) {
	rows, err := s.DB.Query(
		`SELECT id,user_id,chat_id,created_at,service,deadline_raw,pages,topic,requirements,client_source,status,last_receipt_id,last_receipt_type,payment_amount,payment_date,current_board_msg_id,current_board_thread
		   FROM orders
		  WHERE status IN (?, ?, ?)`,
		constants.StatusAwaitPay, constants.StatusReceiptPending, constants.StatusPaid,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []Order
	for rows.Next() {
		var o Order
		if err := scanOrderWithNulls(rows, &o); err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}
	return orders, nil
}

// ↓↓↓ НОВОЕ: для доски (forum topics) ↓↓↓

func (s *Store) UpdateBoardPost(orderID int64, msgID int64, threadID int) error {
	_, err := s.DB.Exec(`UPDATE orders SET current_board_msg_id=?, current_board_thread=? WHERE id=?`, msgID, threadID, orderID)
	return err
}

func (s *Store) GetBoardPost(orderID int64) (msgID int64, threadID int, err error) {
	row := s.DB.QueryRow(`SELECT current_board_msg_id, current_board_thread FROM orders WHERE id=?`, orderID)
	if err = row.Scan(&msgID, &threadID); err != nil {
		return 0, 0, err
	}
	return msgID, threadID, nil
}

// FindOrders ищет по id (если число) или по подстроке в service/faculty/notes
func (s *Store) FindOrders(userQuery string, limit int) ([]Order, error) {
	if limit <= 0 {
		limit = 10
	}

	// Пытаемся как id
	if id, err := strconv.ParseInt(strings.TrimSpace(userQuery), 10, 64); err == nil {
		o, err := s.LoadOrder(id)
		if err == nil && o != nil {
			return []Order{*o}, nil
		}
		// если не нашли по id — падаем в текстовый поиск
	}

	q := "%" + strings.ToLower(strings.TrimSpace(userQuery)) + "%"
	rows, err := s.DB.Query(`
		SELECT id,user_id,chat_id,created_at,service,deadline_raw,pages,topic,requirements,client_source,status,last_receipt_id,last_receipt_type,
		       payment_amount,payment_date,current_board_msg_id,current_board_thread
		  FROM orders
		 WHERE lower(service)  LIKE ?
		    OR lower(topic)    LIKE ?
		    OR lower(requirements) LIKE ?
		 ORDER BY id DESC
		 LIMIT ?`, q, q, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Order
	for rows.Next() {
		var o Order
		if err := scanOrderWithNulls(rows, &o); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, nil
}

func (s *Store) GetOrdersRange(start, end int64, limit int) ([]Order, error) {
	if limit <= 0 {
		limit = 10000
	}
	rows, err := s.DB.Query(`
		SELECT id,user_id,chat_id,created_at,service,deadline_raw,pages,topic,requirements,client_source,status,last_receipt_id,last_receipt_type,
		       payment_amount,payment_date,current_board_msg_id,current_board_thread
		  FROM orders
		 WHERE created_at BETWEEN ? AND ?
		 ORDER BY id DESC
		 LIMIT ?`, start, end, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Order
	for rows.Next() {
		var o Order
		if err := scanOrderWithNulls(rows, &o); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, nil
}

func (s *Store) ListOrdersBetween(start, end int64) ([]Order, error) {
	rows, err := s.DB.Query(`
SELECT id,user_id,chat_id,created_at,service,deadline_raw,pages,topic,requirements,client_source,status,last_receipt_id,last_receipt_type,
       payment_amount,payment_date,current_board_msg_id,current_board_thread
FROM orders
WHERE created_at BETWEEN ? AND ?
ORDER BY id ASC`, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Order
	for rows.Next() {
		var o Order
		if err := scanOrderWithNulls(rows, &o); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, nil
}

func (s *Store) ListAllOrders() ([]Order, error) {
	rows, err := s.DB.Query(`
SELECT id,user_id,chat_id,created_at,service,deadline_raw,pages,topic,requirements,client_source,status,last_receipt_id,last_receipt_type,
       payment_amount,payment_date,current_board_msg_id,current_board_thread
FROM orders
ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Order
	for rows.Next() {
		var o Order
		if err := scanOrderWithNulls(rows, &o); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, nil
}
