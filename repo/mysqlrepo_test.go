package repo_test

import (
	"loginsvc/repo"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMySQLLoginWithName(t *testing.T) {
	repo := repo.GetMySQLLoginRepo()
	sid, err := repo.Name("ed")
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, sid, "a123456789")
}
