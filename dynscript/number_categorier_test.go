package dynscript

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

var numberCategortierRule = `
import = [ "regexp" ]

[[plugins]]
name = "number_categorier"
define = """
m := map[string][]*regexp.Regexp {
"AAA": []*regexp.Regexp{
  regexp.MustCompile("^AAA-.*$"), 
},
"JVR": []*regexp.Regexp{
  regexp.MustCompile("^BBB-.*$"),
},
}
"""
function = """
func(ctx context.Context, number string) (string, bool, error) {
for category, reList := range m {
  for _, re := range reList {
    if re.MatchString(number) {
      return category, true, nil
      }
    }
  }
return "", false, nil
}    
"""
		
`

func TestNumberCategortier(t *testing.T) {
	ctr, err := NewNumberCategorier(numberCategortierRule)
	assert.NoError(t, err)
	m := map[string]string{
		"AAA-123": "AAA",
		"BBB-456": "JVR",
		"CCC-789": "",
	}
	for k, v := range m {
		res, matched, err := ctr.Category(context.Background(), k)
		assert.NoError(t, err)
		if matched {
			assert.Equal(t, v, res)
		} else {
			assert.Equal(t, "", res)
		}
	}
}

var liveNumberCategorierRule = `
import = [ "strings", "regexp" ]

[[plugins]]
name = "basic_categorier"
define = """
cats := map[string][]*regexp.Regexp{
    "FC2": []*regexp.Regexp{
        regexp.MustCompile("(?i)^FC2.*$"),
    },
    "JVR": []*regexp.Regexp{
        regexp.MustCompile("(?i)^JVR.*$"),
    },
    "COSPURI": []*regexp.Regexp{
        regexp.MustCompile("(?i)^COSPURI.*$"),
    },
    "MD": []*regexp.Regexp{
        regexp.MustCompile("(?i)^MADOU[-|_].*$"),
    },
}
"""
function = """
func(ctx context.Context, number string) (string, bool, error) {
    for cat, ruleList := range cats {
        for _, rule := range ruleList {
            if rule.MatchString(number) {
                return cat, true, nil
            }
        }
    }
    return "", false, nil
}
"""
 
`

func TestLiveNumberCategorier(t *testing.T) {
	ctr, err := NewNumberCategorier(liveNumberCategorierRule)
	assert.NoError(t, err)
	m := map[string]string{
		"fc2-ppv-1234":              "FC2",
		"jvr-12345":                 "JVR",
		"qqqq":                      "",
		"HEYZO-12345":               "",
		"COSPURI-Emiri-Momota-0548": "COSPURI",
		"COSPURI-123456":            "COSPURI",
		"cospuri-123456":            "COSPURI",
		"MADOU-123456":              "MD",
		"MADOU_aaaa":                "MD",
		"MADOU_bbbb":                "MD",
	}
	for k, v := range m {
		res, matched, err := ctr.Category(context.Background(), k)
		assert.NoError(t, err)
		if matched {
			assert.Equal(t, v, res)
		} else {
			assert.Equal(t, "", res)
		}
	}
}
