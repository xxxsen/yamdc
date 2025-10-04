package dynscript

import (
	"context"
	"testing"
)

var numberRewriterRule = `
import = [ "regexp" ]

[[plugins]]
name = "rewrite_fc2"
define = """
re := regexp.MustCompile("(?i)^fc2[-|_]?(ppv)?[-|_]?(\\\\d+)([-|_].*)?$")
"""
function = """
func(ctx context.Context, number string) (string, error) {
  number = re.ReplaceAllString(number, "FC2-PPV-$2$3")
  return number, nil 
}
"""

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

const (
	defaultLiveRewriterRule = `
import = [ "strings", "regexp" ]

[[plugins]]
name = "to_upper_string"
function = """
func(ctx context.Context, number string) (string, error) {
    return strings.ToUpper(number), nil 
}
"""

[[plugins]]
name = "basic_number_rewriter"
define = """
sts := []struct{
    Name string
    Rule *regexp.Regexp 
    Rewrite string 
}{
    {
        Name: "format fc2",
        Rule: regexp.MustCompile("(?i)^fc2[-|_]?(ppv)?[-|_]?(\\\\d+)([-|_].*)?$"),
        Rewrite: "FC2-PPV-$2$3",
    },
    {
        Name: "rewrite 1pon or carib",
        Rule: regexp.MustCompile("(?i)(1pondo|1pon|carib)[-|_]?(.*)"),
        Rewrite: "$2",
    },
}
"""
function = """
func(ctx context.Context, number string) (string, error) {
    for _, item := range sts {
        newNumber := item.Rule.ReplaceAllString(number, item.Rewrite)
        number = newNumber 
    }
    return number, nil
}
"""

`
)

func TestLiveRewriterRule(t *testing.T) {
	rewriter, err := NewNumberRewriter(defaultLiveRewriterRule)
	if err != nil {
		t.Fatal(err)
	}

	m := map[string]string{
		"fc2ppv12345-CD1":         "FC2-PPV-12345-CD1",
		"123ABC-456-CD1":          "123ABC-456-CD1",
		"fc2ppv_1234":             "FC2-PPV-1234",
		"fc2_ppv_1234":            "FC2-PPV-1234",
		"fc2ppv-123":              "FC2-PPV-123",
		"fc2-123445-cd1":          "FC2-PPV-123445-CD1",
		"fc2-12345":               "FC2-PPV-12345",
		"aaa":                     "AAA",
		"fc2":                     "FC2",
		"fc2ppv-123-asdasqwe2":    "FC2-PPV-123-ASDASQWE2",
		"fc2ppv-12345-C-CD1":      "FC2-PPV-12345-C-CD1",
		"fc2ppv-12345-CD1":        "FC2-PPV-12345-CD1",
		"123abc_123aaa":           "123ABC_123AAA",
		"123abc_1234":             "123ABC_1234",
		"222aaa-22222_helloworld": "222AAA-22222_HELLOWORLD",
		"aaa-1234-CD1":            "AAA-1234-CD1",
		"carib-1234-222":          "1234-222",
		"1pon-2344-222":           "2344-222",
		"1pondo-1234-222":         "1234-222",
	}
	for k, v := range m {
		res, err := rewriter.Rewrite(context.Background(), k)
		if err != nil {
			t.Fatal(err)
		}
		if res != v {
			t.Fatalf("expected %s for %s, got %s", v, k, res)
		}
	}
}
