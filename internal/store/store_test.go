package store

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := NewStore(t.TempDir() + "/workspaces.json")
	require.NoError(t, err)
	return s
}

func newWS(name string) Workspace {
	return Workspace{
		Name:          name,
		Branch:        name,
		CreatedAt:     time.Now(),
		LastChangedAt: time.Now(),
	}
}

// archiveWS sets ArchivedAt on a store record to simulate archiving.
func archiveWS(t *testing.T, s *Store, id string) {
	t.Helper()
	now := time.Now()
	require.NoError(t, s.Update(id, func(w *Workspace) {
		w.ArchivedAt = &now
	}))
}

func TestNewStore_Empty(t *testing.T) {
	s := newTestStore(t)
	assert.Empty(t, s.List(true))
}

func TestNewStore_LoadsExistingData(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/ws.json"

	s1, err := NewStore(path)
	require.NoError(t, err)
	require.NoError(t, s1.Add(newWS("alpha")))

	s2, err := NewStore(path)
	require.NoError(t, err)
	list := s2.List(false)
	require.Len(t, list, 1)
	assert.Equal(t, "alpha", list[0].Name)
}

func TestAdd_AssignsID(t *testing.T) {
	s := newTestStore(t)
	ws := newWS("foo")
	ws.ID = ""
	require.NoError(t, s.Add(ws))

	all := s.List(true)
	require.Len(t, all, 1)
	assert.NotEmpty(t, all[0].ID)
}

func TestAdd_NameConflict(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.Add(newWS("dup")))
	err := s.Add(newWS("dup"))
	assert.ErrorIs(t, err, ErrNameConflict)
}

func TestAdd_AllowsDuplicateNameIfArchived(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.Add(newWS("reuse")))
	all := s.List(true)
	require.Len(t, all, 1)

	// Archive it by setting ArchivedAt.
	archiveWS(t, s, all[0].ID)

	// Add another with the same name — allowed because the existing one is archived.
	require.NoError(t, s.Add(newWS("reuse")))
}

func TestGet_Happy(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.Add(newWS("getme")))
	list := s.List(true)
	id := list[0].ID

	ws, err := s.Get(id)
	require.NoError(t, err)
	assert.Equal(t, "getme", ws.Name)
}

func TestGet_NotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.Get("nonexistent-id")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestGetByName_Active(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.Add(newWS("byname")))
	ws, err := s.GetByName("byname")
	require.NoError(t, err)
	assert.Equal(t, "byname", ws.Name)
	assert.Nil(t, ws.ArchivedAt, "non-archived workspace should have nil ArchivedAt")
}

func TestGetByName_NotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.GetByName("ghost")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestGetByName_PrefersNonArchived(t *testing.T) {
	s := newTestStore(t)
	// Add then archive a workspace.
	require.NoError(t, s.Add(newWS("prefer")))
	all := s.List(true)
	archiveWS(t, s, all[0].ID)

	// Add a second non-archived workspace with the same name — conflict check allows it
	// because the first is archived.
	require.NoError(t, s.Add(newWS("prefer")))

	ws, err := s.GetByName("prefer")
	require.NoError(t, err)
	assert.Nil(t, ws.ArchivedAt, "GetByName should prefer the non-archived entry")
}

func TestList_ExcludesArchived(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.Add(newWS("active1")))
	require.NoError(t, s.Add(newWS("archived1")))

	all := s.List(true)
	require.Len(t, all, 2)

	// Archive one via ArchivedAt.
	archiveWS(t, s, all[1].ID)

	active := s.List(false)
	require.Len(t, active, 1)
	assert.Equal(t, "active1", active[0].Name)

	withArchived := s.List(true)
	assert.Len(t, withArchived, 2)
}

func TestUpdate_Happy(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.Add(newWS("upd")))
	id := s.List(true)[0].ID

	now := time.Now()
	require.NoError(t, s.Update(id, func(w *Workspace) {
		w.ArchivedAt = &now
	}))

	ws, err := s.Get(id)
	require.NoError(t, err)
	assert.NotNil(t, ws.ArchivedAt)
}

func TestUpdate_NotFound(t *testing.T) {
	s := newTestStore(t)
	err := s.Update("bad-id", func(w *Workspace) {})
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestDelete_Happy(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.Add(newWS("del")))
	id := s.List(true)[0].ID

	require.NoError(t, s.Delete(id))
	assert.Empty(t, s.List(true))

	_, err := s.Get(id)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestDelete_NotFound(t *testing.T) {
	s := newTestStore(t)
	err := s.Delete("ghost")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestUpdateIdleState(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.Add(newWS("idle-ws")))
	id := s.List(true)[0].ID

	now := time.Now()
	require.NoError(t, s.UpdateIdleState(id, "abc123", now))

	ws, err := s.Get(id)
	require.NoError(t, err)
	assert.Equal(t, "abc123", ws.LastCaptureHash)
	assert.Equal(t, now.Unix(), ws.LastChangedAt.Unix())
}

func TestConcurrentAccess(t *testing.T) {
	s := newTestStore(t)
	const workers = 10
	var wg sync.WaitGroup

	// Writers.
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = s.Add(Workspace{
				Name:          "",
				CreatedAt:     time.Now(),
				LastChangedAt: time.Now(),
			})
		}()
	}

	// Readers run concurrently with writers.
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = s.List(false)
		}()
	}

	wg.Wait()
}

func TestConcurrentNamedAdd(t *testing.T) {
	// 10 goroutines all try to add uniquely-named workspaces concurrently.
	s := newTestStore(t)
	const n = 10
	errCh := make(chan error, n)
	var wg sync.WaitGroup

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			errCh <- s.Add(Workspace{
				Name:          generateName(idx),
				Branch:        generateName(idx),
				CreatedAt:     time.Now(),
				LastChangedAt: time.Now(),
			})
		}(i)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		assert.NoError(t, err)
	}

	assert.Len(t, s.List(false), n)
}

func generateName(i int) string {
	names := [10]string{"alpha", "bravo", "charlie", "delta", "echo",
		"foxtrot", "golf", "hotel", "india", "juliet"}
	return names[i]
}
