package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/chudno/zerovibe/internal/domain"
	"github.com/chudno/zerovibe/internal/platform/db"
	"github.com/chudno/zerovibe/internal/repository/sqlite"
)

// newTestUserRepo поднимает временную БД с применёнными миграциями и репозиторий.
func newTestUserRepo(t *testing.T) (*sqlite.UserRepo, *db.DB) {
	t.Helper()
	dsn := "file:" + filepath.Join(t.TempDir(), "u.db")
	database, err := db.Open(dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	if err := database.MigrateUp(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return sqlite.NewUserRepo(database), database
}

// TestUpdateRoleAndEmail_Verified — РЕГРЕССИЯ: переключатель «Почта подтверждена» в
// админке должен сохраняться. Раньше Update игнорировал verified, и галочка не менялась.
func TestUpdateRoleAndEmail_Verified(t *testing.T) {
	repo, _ := newTestUserRepo(t)
	ctx := context.Background()

	u, err := repo.Create(ctx, domain.User{Email: "a@b.com", PasswordHash: "h", Role: domain.RoleUser})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if u.EmailVerified() {
		t.Fatal("новый пользователь не должен быть подтверждён")
	}

	// Включаем подтверждение.
	if err := repo.UpdateRoleAndEmail(ctx, u.ID, "a@b.com", domain.RoleUser, true); err != nil {
		t.Fatalf("update verified=true: %v", err)
	}
	got, _ := repo.ByID(ctx, u.ID)
	if !got.EmailVerified() {
		t.Fatal("после verified=true почта должна быть подтверждена")
	}
	firstVerifiedAt := got.EmailVerifiedAt

	// Повторное сохранение с verified=true НЕ должно перетереть исходную дату.
	if err := repo.UpdateRoleAndEmail(ctx, u.ID, "a@b.com", domain.RoleAdmin, true); err != nil {
		t.Fatalf("update again: %v", err)
	}
	got2, _ := repo.ByID(ctx, u.ID)
	if !got2.EmailVerified() {
		t.Fatal("подтверждение должно сохраниться")
	}
	if !got2.EmailVerifiedAt.Equal(firstVerifiedAt) {
		t.Fatalf("дата подтверждения не должна меняться при повторном сохранении: было %v, стало %v", firstVerifiedAt, got2.EmailVerifiedAt)
	}
	if got2.Role != domain.RoleAdmin {
		t.Fatal("роль должна была обновиться до admin")
	}

	// Снимаем подтверждение.
	if err := repo.UpdateRoleAndEmail(ctx, u.ID, "a@b.com", domain.RoleAdmin, false); err != nil {
		t.Fatalf("update verified=false: %v", err)
	}
	got3, _ := repo.ByID(ctx, u.ID)
	if got3.EmailVerified() {
		t.Fatal("после verified=false почта НЕ должна быть подтверждена")
	}
}
