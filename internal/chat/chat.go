// Package chat 提供会话与消息的本地持久化（SQLite, 纯 Go 驱动）。
package chat

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite" // database/sql 驱动
)

// Role 表示消息角色；值对应 llm.Role，服务层互转。
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// Session 是一个对话会话。
type Session struct {
	ID        string
	Title     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Message 是会话中的一条持久化消息。
type Message struct {
	ID        int64
	Role      Role
	Content   string
	Thinking  string // 模型推理/思考原文（reasoning_content），可为空
	CreatedAt time.Time
}

// Store 抽象会话持久化，便于将来替换实现。
type Store interface {
	CreateSession(title string) (*Session, error)
	ListSessions() ([]Session, error)
	GetSession(id string) (*Session, []Message, error)
	RenameSession(id, title string) error
	DeleteSession(id string) error
	DeleteMessage(sessionID string, id int64) error
	AppendMessage(sessionID string, role Role, content string, thinking string) (Message, error)
	Close() error
}

// SQLiteStore 是基于 SQLite 的 Store 实现。
type SQLiteStore struct {
	db *sql.DB
}

// Open 打开（必要时创建）位于 path 的 SQLite 数据库并完成迁移。
func Open(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("chat: open db: %w", err)
	}
	// SQLite 单写者：限制并发连接并给足忙等待。
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA busy_timeout = 5000`); err != nil {
		db.Close()
		return nil, fmt.Errorf("chat: pragma busy_timeout: %w", err)
	}
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS sessions (
			id         TEXT PRIMARY KEY,
			title      TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		);
		CREATE TABLE IF NOT EXISTS messages (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id TEXT NOT NULL,
			role       TEXT NOT NULL,
			content    TEXT NOT NULL,
			thinking   TEXT NOT NULL DEFAULT '',
			created_at INTEGER NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_messages_session
			ON messages(session_id, id);
	`); err != nil {
		db.Close()
		return nil, fmt.Errorf("chat: migrate: %w", err)
	}
	// 旧库升级：为已存在的 messages 表补 thinking 列（重复添加会被忽略）。
	if _, err := db.Exec(`ALTER TABLE messages ADD COLUMN thinking TEXT NOT NULL DEFAULT ''`); err != nil {
		if !isDupColumn(err) {
			db.Close()
			return nil, fmt.Errorf("chat: add thinking column: %w", err)
		}
	}
	return &SQLiteStore{db: db}, nil
}

func (s *SQLiteStore) Close() error { return s.db.Close() }

// newID 生成随机 hex 会话 ID。
func newID() (string, error) {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

func unixMs(t time.Time) int64 { return t.UnixMilli() }
func fromMs(ms int64) time.Time {
	return time.UnixMilli(ms).UTC()
}

// isDupColumn 判断 SQLite 错误是否为“列已存在”（旧库迁移重复执行时）。
func isDupColumn(err error) bool {
	return err != nil && strings.Contains(err.Error(), "duplicate column name")
}

// CreateSession 新建一个空会话。
func (s *SQLiteStore) CreateSession(title string) (*Session, error) {
	id, err := newID()
	if err != nil {
		return nil, fmt.Errorf("chat: new id: %w", err)
	}
	if title == "" {
		title = "新会话"
	}
	now := time.Now()
	if _, err := s.db.Exec(
		`INSERT INTO sessions (id, title, created_at, updated_at) VALUES (?, ?, ?, ?)`,
		id, title, unixMs(now), unixMs(now),
	); err != nil {
		return nil, fmt.Errorf("chat: insert session: %w", err)
	}
	return &Session{ID: id, Title: title, CreatedAt: now, UpdatedAt: now}, nil
}

// ListSessions 按最近更新倒序返回全部会话。
func (s *SQLiteStore) ListSessions() ([]Session, error) {
	rows, err := s.db.Query(`SELECT id, title, created_at, updated_at FROM sessions ORDER BY updated_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("chat: list sessions: %w", err)
	}
	defer rows.Close()
	var out []Session
	for rows.Next() {
		var ss Session
		var c, u int64
		if err := rows.Scan(&ss.ID, &ss.Title, &c, &u); err != nil {
			return nil, fmt.Errorf("chat: scan session: %w", err)
		}
		ss.CreatedAt, ss.UpdatedAt = fromMs(c), fromMs(u)
		out = append(out, ss)
	}
	return out, rows.Err()
}

// GetSession 返回会话及其全部消息（按时间升序）。
func (s *SQLiteStore) GetSession(id string) (*Session, []Message, error) {
	var ss Session
	var c, u int64
	err := s.db.QueryRow(
		`SELECT id, title, created_at, updated_at FROM sessions WHERE id = ?`, id,
	).Scan(&ss.ID, &ss.Title, &c, &u)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, fmt.Errorf("chat: session %q not found", id)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("chat: get session: %w", err)
	}
	ss.CreatedAt, ss.UpdatedAt = fromMs(c), fromMs(u)

	rows, err := s.db.Query(
		`SELECT id, role, content, thinking, created_at FROM messages WHERE session_id = ? ORDER BY id`, id,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("chat: list messages: %w", err)
	}
	defer rows.Close()
	var msgs []Message
	for rows.Next() {
		var m Message
		var t int64
		var role string
		if err := rows.Scan(&m.ID, &role, &m.Content, &m.Thinking, &t); err != nil {
			return nil, nil, fmt.Errorf("chat: scan message: %w", err)
		}
		m.Role, m.CreatedAt = Role(role), fromMs(t)
		msgs = append(msgs, m)
	}
	return &ss, msgs, rows.Err()
}

// RenameSession 更新会话标题。
func (s *SQLiteStore) RenameSession(id, title string) error {
	if title == "" {
		return errors.New("chat: empty title")
	}
	res, err := s.db.Exec(
		`UPDATE sessions SET title = ?, updated_at = ? WHERE id = ?`, title, unixMs(time.Now()), id,
	)
	if err != nil {
		return fmt.Errorf("chat: rename session: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("chat: session %q not found", id)
	}
	return nil
}

// DeleteSession 删除会话及其消息。
func (s *SQLiteStore) DeleteSession(id string) error {
	if _, err := s.db.Exec(`DELETE FROM messages WHERE session_id = ?`, id); err != nil {
		return fmt.Errorf("chat: delete messages: %w", err)
	}
	if _, err := s.db.Exec(`DELETE FROM sessions WHERE id = ?`, id); err != nil {
		return fmt.Errorf("chat: delete session: %w", err)
	}
	return nil
}

// DeleteMessage 删除会话内的单条消息（id 为该消息在 messages 表中的主键）。
func (s *SQLiteStore) DeleteMessage(sessionID string, id int64) error {
	res, err := s.db.Exec(`DELETE FROM messages WHERE id = ? AND session_id = ?`, id, sessionID)
	if err != nil {
		return fmt.Errorf("chat: delete message: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("chat: message %d not found in session %q", id, sessionID)
	}
	return nil
}

// AppendMessage 追加一条消息并刷新会话更新时间。
func (s *SQLiteStore) AppendMessage(sessionID string, role Role, content string, thinking string) (Message, error) {
	if role != RoleUser && role != RoleAssistant {
		return Message{}, fmt.Errorf("chat: invalid role %q", role)
	}
	now := time.Now()
	m := Message{Role: role, Content: content, Thinking: thinking, CreatedAt: now}

	tx, err := s.db.Begin()
	if err != nil {
		return Message{}, fmt.Errorf("chat: begin tx: %w", err)
	}
	defer tx.Rollback()

	res, err := tx.Exec(
		`INSERT INTO messages (session_id, role, content, thinking, created_at) VALUES (?, ?, ?, ?, ?)`,
		sessionID, string(role), content, thinking, unixMs(now),
	)
	if err != nil {
		return Message{}, fmt.Errorf("chat: insert message: %w", err)
	}
	m.ID, _ = res.LastInsertId()

	if _, err := tx.Exec(
		`UPDATE sessions SET updated_at = ? WHERE id = ?`, unixMs(now), sessionID,
	); err != nil {
		return Message{}, fmt.Errorf("chat: touch session: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Message{}, fmt.Errorf("chat: commit: %w", err)
	}
	return m, nil
}
