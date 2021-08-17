package repo_test

import (
	"loginsvc/repo"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoginWithName(t *testing.T) {
	repo := repo.GetSqliteLoginRepository()
	sid, err := repo.Name("ed")
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, sid, "a123456789")
}
