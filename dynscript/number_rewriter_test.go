package dynscript

import (
	"context"
	"testing"
)

var numberRewriterRule = `
plugins:
  - name: rewrite_fc2
    define: |
      re := regexp.MustCompile("(?i)^fc2[-|_]?(ppv)?[-|_]?(\\d+)([-|_].*)?$")
    function: |
      func(ctx context.Context, number string) (string, error) {
        number = re.ReplaceAllString(number, "FC2-PPV-$2$3")
        return number, nil 
      }

import:
  - regexp  
`

func TestNumberRewrite(t *testing.T) {
	rewriter, err := NewNumberRewriter(numberRewriterRule)
	if err != nil {
		t.Fatal(err)
	}
	res, err := rewriter.Rewrite(context.Background(), "fc2ppv12345-CD1")
	if err != nil {
		t.Fatal(err)
	}
	if res != "FC2-PPV-12345-CD1" {
		t.Fatalf("expected FC2-PPV-12345-CD1, got %s", res)
	}
}
