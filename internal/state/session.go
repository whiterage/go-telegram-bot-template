package state

import "sync"

type Stage int

const (
	StageIdle Stage = iota
	StageWelcome
	StageChooseInstitution
	StageAskDeadline
	StageChooseWorkCategory // ← НОВОЕ: выбор категории работ
	StageChooseWorkType     // ← обновлено: выбор конкретного типа
	StageAskPages
	StageAskTopic
	StageAskRequirements // ← НОВОЕ: ввод требований
	StageAskDocs
	StageAskClientSource
	StageConfirm
	StageAwaitReceiptUpload // ← НОВОЕ: ждём загрузку чека
)

type Session struct {
	Stage           Stage
	InstitutionType string
	Deadline        string
	WorkCategory    string // ← НОВОЕ: выбранная категория работ
	WorkType        string
	Pages           string
	Topic           string // ← НОВОЕ: тема работы
	Requirements    string // ← НОВОЕ: требования
	ClientSource    string
	AttachIDs       []string

	AwaitReceiptFor int64 // ← НОВОЕ: к какой заявке ждём чек
}

type Sessions struct {
	mu sync.RWMutex
	m  map[int64]*Session
}

func NewSessions() *Sessions { return &Sessions{m: make(map[int64]*Session)} }

func (s *Sessions) Get(chatID int64) *Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess, ok := s.m[chatID]; ok {
		return sess
	}
	newSess := &Session{Stage: StageIdle}
	s.m[chatID] = newSess
	return newSess
}

func (s *Sessions) Set(chatID int64, sess *Session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[chatID] = sess
}

func (s *Sessions) Reset(chatID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, chatID)
}
