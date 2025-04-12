package dynscript

import (
	"context"
	"testing"
)

var numberUncensorRule = `
plugins:
  - name: check_uncensor
    define: |
      prefixList := []string {
          "FC2",
          "HEYZO",
      }
      reList := []*regexp.Regexp{
          regexp.MustCompile("(?i)^AAA-.*$"),
      }		
    function: |
      func(ctx context.Context, number string) (bool, error) {
          for _, prefix := range prefixList {
              if strings.HasPrefix(number, prefix) {
                  return true, nil
              }
          }
          for _, re := range reList {
              if re.MatchString(number) {
                  return true, nil
              }
          }
          return false, nil	  			
      }

import:
  - regexp  
  - strings
`

func TestNumberUncensorCheck(t *testing.T) {
	ck, err := NewNumberUncensorChecker(numberUncensorRule)
	if err != nil {
		t.Fatal(err)
	}
	m := map[string]bool{
		"FC2-PPV-123": true,
		"HEYZO-222":   true,
		"AAA-22222":   true,
		"BVBBB-22222": false,
	}
	for k, v := range m {
		res, err := ck.IsMatch(context.Background(), k)
		if err != nil {
			t.Fatal(err)
		}
		if res != v {
			t.Fatalf("expected %v for %s, got %v", v, k, res)
		}
	}
}
