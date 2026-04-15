package translator_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/xxxsen/yamdc/internal/translator"
)

func TestConstants(t *testing.T) {
	require.Equal(t, "google", translator.TrNameGoogle)
	require.Equal(t, "ai", translator.TrNameAI)
}
