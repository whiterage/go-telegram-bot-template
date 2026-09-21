package storage

import (
	"path/filepath"
	"testing"
)

// Новая база должна открываться с первого раза: initSchema создаёт таблицу
// сразу по актуальной схеме, поэтому исторические миграции по ней гнать нельзя.
func TestOpenFreshDB(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fresh.db")

	st, err := Open(path)
	if err != nil {
		t.Fatalf("открытие новой базы упало: %v", err)
	}
	defer st.DB.Close()

	var version int
	if err := st.DB.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatalf("чтение user_version: %v", err)
	}
	if version != CurrentSchemaVersion {
		t.Errorf("user_version = %d, ожидалось %d", version, CurrentSchemaVersion)
	}

	// Мусора от миграции 4 остаться не должно
	var name string
	err = st.DB.QueryRow(
		`SELECT name FROM sqlite_master WHERE type='table' AND name='orders_new'`,
	).Scan(&name)
	if err == nil {
		t.Error("в базе осталась временная таблица orders_new")
	}
}

// Повторное открытие не должно ничего ломать.
func TestOpenIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "twice.db")

	st, err := Open(path)
	if err != nil {
		t.Fatalf("первое открытие: %v", err)
	}
	if _, err := st.DB.Exec(
		`INSERT INTO orders (user_id, chat_id, created_at, service, status)
		 VALUES (1, 1, 0, 'Курсовая', 'new')`); err != nil {
		t.Fatalf("вставка заявки: %v", err)
	}
	st.DB.Close()

	st2, err := Open(path)
	if err != nil {
		t.Fatalf("повторное открытие: %v", err)
	}
	defer st2.DB.Close()

	count, err := st2.GetOrdersCount()
	if err != nil {
		t.Fatalf("подсчёт заявок: %v", err)
	}
	if count != 1 {
		t.Errorf("заявок в базе %d, ожидалась 1", count)
	}
}

// База, заклинившая из-за старого бага: таблица orders уже по новой схеме,
// user_version=0, а от упавшей миграции 4 остался огрызок orders_new.
// Open должен её починить, а не падать на "table orders_new already exists".
func TestOpenRecoversWedgedDB(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wedged.db")

	st, err := Open(path)
	if err != nil {
		t.Fatalf("подготовка базы: %v", err)
	}
	// Воспроизводим состояние после падения миграции
	if _, err := st.DB.Exec(`CREATE TABLE orders_new (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatalf("создание orders_new: %v", err)
	}
	if _, err := st.DB.Exec(`PRAGMA user_version = 0`); err != nil {
		t.Fatalf("сброс user_version: %v", err)
	}
	st.DB.Close()

	st2, err := Open(path)
	if err != nil {
		t.Fatalf("база не восстановилась: %v", err)
	}
	defer st2.DB.Close()

	var version int
	if err := st2.DB.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatalf("чтение user_version: %v", err)
	}
	if version != CurrentSchemaVersion {
		t.Errorf("user_version = %d, ожидалось %d", version, CurrentSchemaVersion)
	}
}
