package service

import (
	"sync"

	"cdpnetool/internal/cdp"
	ilog "cdpnetool/internal/log"
	"cdpnetool/pkg/errx"
	"cdpnetool/pkg/model"
)

type svc struct {
	mu       sync.Mutex
	sessions map[model.SessionID]*session
}

type session struct {
	id      model.SessionID
	cfg     model.SessionConfig
	rules   model.RuleSet
	events  chan model.Event
	pending chan any
	mgr     *cdp.Manager
}

func New() *svc {
	return &svc{sessions: make(map[model.SessionID]*session)}
}

func (s *svc) StartSession(cfg model.SessionConfig) (model.SessionID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := model.SessionID(generateID())
	ses := &session{
		id:      id,
		cfg:     cfg,
		events:  make(chan model.Event, 128),
		pending: make(chan any, cfg.PendingCapacity),
	}
	ses.mgr = cdp.New(cfg.DevToolsURL, ses.events, ses.pending)
	ses.mgr.SetConcurrency(cfg.Concurrency)
	ses.mgr.SetRuntime(cfg.BodySizeThreshold, cfg.ProcessTimeoutMS)
	s.sessions[id] = ses
	ilog.L().Info("start_session", "session", string(id), "devtools", cfg.DevToolsURL, "concurrency", cfg.Concurrency, "pending", cfg.PendingCapacity)
	return id, nil
}

func (s *svc) StopSession(id model.SessionID) error {
	s.mu.Lock()
	ses, ok := s.sessions[id]
	if ok {
		delete(s.sessions, id)
	}
	s.mu.Unlock()
	if !ok {
		return errx.New(errx.CodeSessionNotFound, "session not found")
	}
	close(ses.events)
	close(ses.pending)
	if ses.mgr != nil {
		ses.mgr.Detach()
	}
	ilog.L().Info("stop_session", "session", string(id))
	return nil
}

func (s *svc) AttachTarget(id model.SessionID, target model.TargetID) error {
	s.mu.Lock()
	ses, ok := s.sessions[id]
	s.mu.Unlock()
	if !ok {
		return errx.New(errx.CodeSessionNotFound, "session not found")
	}
	if ses.mgr == nil {
		ses.mgr = cdp.New(ses.cfg.DevToolsURL, ses.events, ses.pending)
		ses.mgr.SetConcurrency(ses.cfg.Concurrency)
		ses.mgr.SetRuntime(ses.cfg.BodySizeThreshold, ses.cfg.ProcessTimeoutMS)
	}
	err := ses.mgr.AttachTarget(target)
	if err == nil {
		ilog.L().Info("attach_target", "session", string(id), "target", string(target))
	}
	return err
}

func (s *svc) DetachTarget(id model.SessionID, target model.TargetID) error {
	s.mu.Lock()
	ses, ok := s.sessions[id]
	s.mu.Unlock()
	if !ok {
		return errx.New(errx.CodeSessionNotFound, "session not found")
	}
	if ses.mgr != nil {
		return ses.mgr.Detach()
	}
	return nil
}

func (s *svc) EnableInterception(id model.SessionID) error {
	s.mu.Lock()
	ses, ok := s.sessions[id]
	s.mu.Unlock()
	if !ok {
		return errx.New(errx.CodeSessionNotFound, "session not found")
	}
	if ses.mgr == nil {
		return errx.New(errx.CodeSessionNotFound, "manager not initialized")
	}
	err := ses.mgr.Enable()
	if err == nil {
		ilog.L().Info("enable_interception", "session", string(id))
	}
	return err
}

func (s *svc) DisableInterception(id model.SessionID) error {
	s.mu.Lock()
	ses, ok := s.sessions[id]
	s.mu.Unlock()
	if !ok {
		return errx.New(errx.CodeSessionNotFound, "session not found")
	}
	if ses.mgr == nil {
		return errx.New(errx.CodeSessionNotFound, "manager not initialized")
	}
	err := ses.mgr.Disable()
	if err == nil {
		ilog.L().Info("disable_interception", "session", string(id))
	}
	return err
}

func (s *svc) LoadRules(id model.SessionID, rs model.RuleSet) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ses, ok := s.sessions[id]
	if !ok {
		return errx.New(errx.CodeSessionNotFound, "session not found")
	}
	ses.rules = rs
	ilog.L().Info("load_rules", "session", string(id), "count", len(rs.Rules), "version", rs.Version)
	if ses.mgr != nil {
		ses.mgr.UpdateRules(rs)
	}
	return nil
}

func (s *svc) GetRuleStats(id model.SessionID) (model.EngineStats, error) {
	return model.EngineStats{ByRule: make(map[model.RuleID]int64)}, nil
}

func (s *svc) SubscribeEvents(id model.SessionID) (<-chan model.Event, error) {
	s.mu.Lock()
	ses, ok := s.sessions[id]
	s.mu.Unlock()
	if !ok {
		return nil, errx.New(errx.CodeSessionNotFound, "session not found")
	}
	return ses.events, nil
}

func (s *svc) SubscribePending(id model.SessionID) (<-chan any, error) {
	s.mu.Lock()
	ses, ok := s.sessions[id]
	s.mu.Unlock()
	if !ok {
		return nil, errx.New(errx.CodeSessionNotFound, "session not found")
	}
	return ses.pending, nil
}

func (s *svc) ApproveRequest(itemID string, mutations model.Rewrite) error {
	s.mu.Lock()
	for _, ses := range s.sessions {
		if ses.mgr != nil {
			ses.mgr.Approve(itemID, mutations)
		}
	}
	s.mu.Unlock()
	return nil
}

func (s *svc) ApproveResponse(itemID string, mutations model.Rewrite) error {
	s.mu.Lock()
	for _, ses := range s.sessions {
		if ses.mgr != nil {
			ses.mgr.Approve(itemID, mutations)
		}
	}
	s.mu.Unlock()
	return nil
}

func (s *svc) Reject(itemID string) error {
	return nil
}

func generateID() string {
	return randomID()
}

func randomID() string {
	var alphabet = []byte("abcdefghijklmnopqrstuvwxyz0123456789")
	b := make([]byte, 16)
	for i := range b {
		b[i] = alphabet[i%len(alphabet)]
	}
	return string(b)
}
