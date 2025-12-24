package config

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tailscale/hujson"
)

const (
	testData = `
{
	/* this is a test comment */
	"a": 1,
	"b": 3.14, // hello?
	"c": true,
	//also comment here
	"d": ["a", "b"], //asdasdsadasd
}
	`
)

type testSt struct {
	A int      `json:"a"`
	B float64  `json:"b"`
	C bool     `json:"c"`
	D []string `json:"d"`
}

func TestJsonWithComments(t *testing.T) {
	st := &testSt{}
	data, err := hujson.Standardize([]byte(testData))
	assert.NoError(t, err)
	err = json.Unmarshal(data, st)
	assert.NoError(t, err)
	t.Logf("%+v", *st)
	assert.Equal(t, 1, st.A)
	assert.Equal(t, 3.14, st.B)
	assert.Equal(t, true, st.C)
}
