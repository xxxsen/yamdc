package downloadmgr

import (
	"testing"
	"yamdc/client"

	"github.com/stretchr/testify/assert"
)

func TestDownloa(t *testing.T) {
	m := NewManager(client.MustNewClient())
	err := m.Download("https://github.com/Kagami/go-face-testdata/raw/master/models/shape_predictor_5_face_landmarks.dat", "testdata/abc.dat")
	assert.NoError(t, err)
}
