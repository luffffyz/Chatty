package chat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func openTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestSessionLifecycle(t *testing.T) {
	st := openTestStore(t)

	s, err := st.CreateSession("")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if s.ID == "" {
		t.Fatal("empty session id")
	}

	// 追加消息
	if _, err := st.AppendMessage(s.ID, RoleUser, "hello"); err != nil {
		t.Fatalf("AppendMessage(user): %v", err)
	}
	if _, err := st.AppendMessage(s.ID, RoleAssistant, "hi there"); err != nil {
		t.Fatalf("AppendMessage(assistant): %v", err)
	}

	got, msgs, err := st.GetSession(s.ID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got.ID != s.ID || got.Title != "新会话" {
		t.Errorf("session = %+v", got)
	}
	if len(msgs) != 2 {
		t.Fatalf("messages = %d, want 2", len(msgs))
	}
	if msgs[0].Role != RoleUser || msgs[0].Content != "hello" {
		t.Errorf("msg[0] = %+v", msgs[0])
	}
	if msgs[1].Role != RoleAssistant || msgs[1].Content != "hi there" {
		t.Errorf("msg[1] = %+v", msgs[1])
	}
	if msgs[0].ID >= msgs[1].ID {
		t.Errorf("message ids not increasing: %d >= %d", msgs[0].ID, msgs[1].ID)
	}

	// 重命名
	if err := st.RenameSession(s.ID, "标题"); err != nil {
		t.Fatalf("RenameSession: %v", err)
	}
	got, _, _ = st.GetSession(s.ID)
	if got.Title != "标题" {
		t.Errorf("title = %q", got.Title)
	}

	// 列出
	all, err := st.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(all) != 1 || all[0].ID != s.ID {
		t.Errorf("list = %+v", all)
	}

	// 删除后消失
	if err := st.DeleteSession(s.ID); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	if _, _, err := st.GetSession(s.ID); err == nil {
		t.Error("GetSession after delete should error")
	}
	all, _ = st.ListSessions()
	if len(all) != 0 {
		t.Errorf("sessions after delete = %d", len(all))
	}
}

func TestPersistenceAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "persist.db")

	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	s, _ := st.CreateSession("keep me")
	_, _ = st.AppendMessage(s.ID, RoleUser, "persisted")
	st.Close()

	st2, err := Open(path)
	if err != nil {
		t.Fatalf("Reopen: %v", err)
	}
	defer st2.Close()
	got, msgs, err := st2.GetSession(s.ID)
	if err != nil {
		t.Fatalf("GetSession after reopen: %v", err)
	}
	if got.Title != "keep me" || len(msgs) != 1 || msgs[0].Content != "persisted" {
		t.Errorf("after reopen: session=%+v msgs=%+v", got, msgs)
	}
}

func TestInvalidRoleRejected(t *testing.T) {
	st := openTestStore(t)
	s, _ := st.CreateSession("t")
	if _, err := st.AppendMessage(s.ID, Role("bogus"), "x"); err == nil {
		t.Fatal("expected error for invalid role")
	}
}

func TestOpenBadPath(t *testing.T) {
	// 目录不存在时应报错（默认 sqlite 会尝试创建文件；传一个无效目录路径）
	if _, err := Open(filepath.Join(os.TempDir(), "no_such_dir_xyz", "a.db")); err == nil {
		t.Fatal("expected error opening db in missing directory")
	} else if !strings.Contains(err.Error(), "chat:") {
		t.Errorf("error not wrapped: %v", err)
	}
}
