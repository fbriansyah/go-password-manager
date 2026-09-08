package tui

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/fbriansyah/go-password-manager/internal/config"
	"github.com/fbriansyah/go-password-manager/internal/crypto"
	"github.com/fbriansyah/go-password-manager/internal/secret"
	"github.com/fbriansyah/go-password-manager/internal/vault"
)

const testPassword = "master-password"

// unlocked menyiapkan Vault berisi satu Secret lalu menjalankan alur Unlock
// seperti yang dilakukan pengguna: mengetik password lalu menekan enter.
func unlocked(t *testing.T) Model {
	t.Helper()
	keyDir, vaultDir := t.TempDir(), t.TempDir()
	idPath := filepath.Join(keyDir, "identity.age")
	recPath := filepath.Join(keyDir, "recipient.pub")
	if err := crypto.GenerateKeypair(idPath, recPath, testPassword); err != nil {
		t.Fatalf("GenerateKeypair: %v", err)
	}
	session, err := crypto.Unlock(idPath, testPassword)
	if err != nil {
		t.Fatalf("Unlock: %v", err)
	}
	store, err := vault.Open(vaultDir, session)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := store.Create(&secret.Secret{
		Meta: secret.Meta{Title: "Facebook", Description: "Facebook Credential", Tags: []string{"app"}},
		Fields: []secret.Field{
			{Type: "tx", Label: "Username", Value: "budi@mail.com"},
			{Type: "ps", Label: "Password", Value: "rahasia123"},
		},
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	m := New(vaultDir, config.Config{PrivateKeyPath: idPath, PublicKeyPath: recPath})
	m = update(t, m, tea.WindowSizeMsg{Width: 120, Height: 32})
	m = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(testPassword)})
	m = run(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.screen != screenList {
		t.Fatalf("screen = %v, mau screenList", m.screen)
	}
	return m
}

// update mengirim satu pesan dan mengabaikan perintah yang dihasilkan.
func update(t *testing.T, m Model, msg tea.Msg) Model {
	t.Helper()
	next, _ := m.Update(msg)
	return next.(Model)
}

// run mengirim satu pesan lalu menjalankan perintah yang dihasilkannya dan
// mengumpankan hasilnya kembali — pengganti loop bubbletea di pengujian.
func run(t *testing.T, m Model, msg tea.Msg) Model {
	t.Helper()
	next, cmd := m.Update(msg)
	m = next.(Model)
	if cmd == nil {
		return m
	}
	out := cmd()
	if out == nil {
		return m
	}
	next, _ = m.Update(out)
	return next.(Model)
}

func TestUnlockMemuatDaftarSecret(t *testing.T) {
	m := unlocked(t)
	if len(m.list.Items()) != 1 {
		t.Fatalf("jumlah item = %d, mau 1", len(m.list.Items()))
	}
	view := m.View()
	if !strings.Contains(view, "Facebook") {
		t.Fatalf("judul tidak tampil di layar:\n%s", view)
	}
	if !strings.Contains(view, "Username") {
		t.Fatalf("panel detail tidak menampilkan field:\n%s", view)
	}
}

func TestPasswordSalahTetapDiLayarUnlock(t *testing.T) {
	m := unlocked(t)
	m.screen = screenUnlock
	m.unlock = newUnlock(m.vaultDir)
	m = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("salah-sekali")})
	m = run(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.screen != screenUnlock {
		t.Fatal("password salah seharusnya tidak membuka vault")
	}
	if m.unlock.err == nil {
		t.Fatal("mau pesan kesalahan di layar unlock")
	}
	if strings.Contains(m.View(), "salah-sekali") {
		t.Fatal("password yang diketik tampil di layar")
	}
}

func TestPasswordDisembunyikanSampaiDitekanR(t *testing.T) {
	m := unlocked(t)
	m = update(t, m, tea.KeyMsg{Type: tea.KeyTab})                       // fokus ke panel detail
	m = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}) // ke field Password
	if strings.Contains(m.View(), "rahasia123") {
		t.Fatal("password tampil tanpa diminta")
	}
	m = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	if !strings.Contains(m.View(), "rahasia123") {
		t.Fatalf("r tidak memperlihatkan nilai:\n%s", m.View())
	}
	// Berpindah field menutup kembali nilai yang tadi diperlihatkan.
	m = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if strings.Contains(m.View(), "rahasia123") {
		t.Fatal("nilai tetap terbuka setelah pindah field")
	}
}

func TestBuatSecretBaruLewatForm(t *testing.T) {
	m := unlocked(t)
	m = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	if m.screen != screenForm {
		t.Fatalf("screen = %v, mau screenForm", m.screen)
	}
	m = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Gmail")})
	m = update(t, m, tea.KeyMsg{Type: tea.KeyTab}) // deskripsi
	m = update(t, m, tea.KeyMsg{Type: tea.KeyTab}) // tag
	m = update(t, m, tea.KeyMsg{Type: tea.KeyTab}) // label field pertama
	m = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Password")})
	m = update(t, m, tea.KeyMsg{Type: tea.KeyRight}) // tx -> ps
	m = update(t, m, tea.KeyMsg{Type: tea.KeyTab})   // nilai
	m = update(t, m, tea.KeyMsg{Type: tea.KeyCtrlG}) // generate

	m = run(t, m, tea.KeyMsg{Type: tea.KeyCtrlS})
	if m.screen != screenList {
		t.Fatalf("screen = %v setelah simpan, mau screenList (err: %v)", m.screen, m.form.err)
	}
	if len(m.list.Items()) != 2 {
		t.Fatalf("jumlah item = %d, mau 2", len(m.list.Items()))
	}
	s, err := m.store.Load("gmail")
	if err != nil {
		t.Fatalf("Load gmail: %v", err)
	}
	if len(s.Fields) != 1 || s.Fields[0].Type != "ps" || s.Fields[0].Label != "Password" {
		t.Fatalf("field tersimpan salah: %+v", s.Fields)
	}
	if len(s.Fields[0].Value) != 20 {
		t.Fatalf("generator tidak mengisi nilai: %q", s.Fields[0].Value)
	}
}

func TestFormMenolakSecretTanpaJudul(t *testing.T) {
	m := unlocked(t)
	m = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	m = run(t, m, tea.KeyMsg{Type: tea.KeyCtrlS})
	if m.screen != screenForm {
		t.Fatal("form tanpa judul seharusnya tidak tersimpan")
	}
	if m.form.err == nil {
		t.Fatal("mau pesan kesalahan di form")
	}
}

func TestJudulDuplikatDitolakDenganPesanJelas(t *testing.T) {
	m := unlocked(t)
	m = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	m = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Facebook")})
	m = run(t, m, tea.KeyMsg{Type: tea.KeyCtrlS})
	if m.screen != screenForm {
		t.Fatal("judul duplikat seharusnya tidak tersimpan")
	}
	if m.form.err == nil || !strings.Contains(m.form.err.Error(), "sudah ada") {
		t.Fatalf("pesan kesalahan tidak menjelaskan tabrakan: %v", m.form.err)
	}
}
