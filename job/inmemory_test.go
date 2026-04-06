package job

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	cronlib "github.com/robfig/cron/v3"
)

func TestInMemoryRepository_Add(t *testing.T) {
	repo := NewInMemoryRepository()

	info := &Info{JobId: "job1", Spec: "* * * * *", EntryId: cronlib.EntryID(1)}
	err := repo.Add(info)
	require.NoError(t, err)

	got, ok := repo.Get("job1")
	require.True(t, ok)
	assert.Equal(t, "job1", got.JobId)
	assert.Equal(t, "* * * * *", got.Spec)
}

func TestInMemoryRepository_Add_Duplicate(t *testing.T) {
	repo := NewInMemoryRepository()

	info := &Info{JobId: "job1", Spec: "* * * * *", EntryId: cronlib.EntryID(1)}
	_ = repo.Add(info)

	err := repo.Add(info)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestInMemoryRepository_Add_Nil(t *testing.T) {
	repo := NewInMemoryRepository()

	err := repo.Add(nil)
	assert.Error(t, err)
}

func TestInMemoryRepository_Add_EmptyJobId(t *testing.T) {
	repo := NewInMemoryRepository()

	info := &Info{JobId: "", Spec: "* * * * *", EntryId: cronlib.EntryID(1)}
	err := repo.Add(info)
	assert.Error(t, err)
}

func TestInMemoryRepository_Get_NotFound(t *testing.T) {
	repo := NewInMemoryRepository()

	_, ok := repo.Get("nonexistent")
	assert.False(t, ok)
}

func TestInMemoryRepository_Remove(t *testing.T) {
	repo := NewInMemoryRepository()

	info := &Info{JobId: "job1", Spec: "* * * * *", EntryId: cronlib.EntryID(1)}
	_ = repo.Add(info)

	err := repo.Remove("job1")
	require.NoError(t, err)

	_, ok := repo.Get("job1")
	assert.False(t, ok)
}

func TestInMemoryRepository_Remove_NotFound(t *testing.T) {
	repo := NewInMemoryRepository()

	err := repo.Remove("nonexistent")
	assert.Error(t, err)
}

func TestInMemoryRepository_List(t *testing.T) {
	repo := NewInMemoryRepository()

	info1 := &Info{JobId: "job1", Spec: "* * * * *", EntryId: cronlib.EntryID(1)}
	info2 := &Info{JobId: "job2", Spec: "*/2 * * * *", EntryId: cronlib.EntryID(2)}
	_ = repo.Add(info1)
	_ = repo.Add(info2)

	list := repo.List()
	assert.Len(t, list, 2)
}

func TestInMemoryRepository_Len(t *testing.T) {
	repo := NewInMemoryRepository()

	assert.Equal(t, 0, repo.Len())

	info := &Info{JobId: "job1", Spec: "* * * * *", EntryId: cronlib.EntryID(1)}
	_ = repo.Add(info)

	assert.Equal(t, 1, repo.Len())
}

func TestInMemoryRepository_Clear(t *testing.T) {
	repo := NewInMemoryRepository()

	info := &Info{JobId: "job1", Spec: "* * * * *", EntryId: cronlib.EntryID(1)}
	_ = repo.Add(info)

	repo.Clear()
	assert.Equal(t, 0, repo.Len())
}

func TestInMemoryRepository_Exists(t *testing.T) {
	repo := NewInMemoryRepository()

	info := &Info{JobId: "job1", Spec: "* * * * *", EntryId: cronlib.EntryID(1)}
	_ = repo.Add(info)

	assert.True(t, repo.Exists("job1"))
	assert.False(t, repo.Exists("nonexistent"))
}

func TestInMemoryRepository_Concurrent(t *testing.T) {
	repo := NewInMemoryRepository()
	const workers = 100

	done := make(chan bool)

	for i := 0; i < workers; i++ {
		go func(id int) {
			info := &Info{
				JobId:   fmt.Sprintf("job%d", id),
				Spec:    "* * * * *",
				EntryId: cronlib.EntryID(id),
			}
			_ = repo.Add(info)
			done <- true
		}(i)
	}

	for i := 0; i < workers; i++ {
		<-done
	}

	assert.Equal(t, workers, repo.Len())
}
