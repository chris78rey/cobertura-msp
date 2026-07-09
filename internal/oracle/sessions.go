package oracle

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// SessionManager maneja conexiones Oracle por usuario
type SessionManager struct {
	mu       sync.RWMutex
	sessions map[string]*UserSession
}

// UserSession representa la sesión de un usuario autenticado
type UserSession struct {
	Username   string
	DB         *sql.DB
	Token      string
	CreatedAt  time.Time
	LastAccess time.Time
}

// NewSessionManager crea un gestor de sesiones con limpieza periódica
func NewSessionManager() *SessionManager {
	sm := &SessionManager{
		sessions: make(map[string]*UserSession),
	}
	// Limpiar sesiones inactivas cada 5 minutos
	go sm.cleanupLoop()
	return sm
}

// Login valida credenciales contra Oracle y crea una sesión
func (sm *SessionManager) Login(username, password string) (*UserSession, error) {
	// Probar conexión con las credenciales del usuario
	connStr := buildOracleDSN(username, password, "172.16.60.20", "prdsgh")
	db, err := sql.Open("oracle", connStr)
	if err != nil {
		return nil, fmt.Errorf("error de conexión: %w", err)
	}
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(30 * time.Minute)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("credenciales inválidas: %w", err)
	}

	token := generateToken()
	session := &UserSession{
		Username:   username,
		DB:         db,
		Token:      token,
		CreatedAt:  time.Now(),
		LastAccess: time.Now(),
	}

	sm.mu.Lock()
	// Cerrar sesión anterior del mismo usuario si existe
	for t, s := range sm.sessions {
		if s.Username == username {
			s.DB.Close()
			delete(sm.sessions, t)
		}
	}
	sm.sessions[token] = session
	sm.mu.Unlock()

	return session, nil
}

// GetSession obtiene una sesión por token
func (sm *SessionManager) GetSession(token string) *UserSession {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	s, ok := sm.sessions[token]
	if ok {
		s.LastAccess = time.Now()
	}
	return s
}

// Logout cierra la sesión y la conexión Oracle
func (sm *SessionManager) Logout(token string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if s, ok := sm.sessions[token]; ok {
		s.DB.Close()
		delete(sm.sessions, token)
	}
}

func (sm *SessionManager) cleanupLoop() {
	for {
		time.Sleep(5 * time.Minute)
		sm.mu.Lock()
		cutoff := time.Now().Add(-30 * time.Minute)
		for token, s := range sm.sessions {
			if s.LastAccess.Before(cutoff) {
				s.DB.Close()
				delete(sm.sessions, token)
			}
		}
		sm.mu.Unlock()
	}
}

func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}
