package dynscript

import (
	"context"
	"testing"
)

var numberUncensorRule = `
import = [ "regexp", "strings" ]

[[plugins]]
name = "check_uncensor"
define = """
prefixList := []string {
    "FC2",
    "HEYZO",
}
reList := []*regexp.Regexp{
    regexp.MustCompile("(?i)^AAA-.*$"),
}
"""
function = """
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
    return false, nil\t  \t\t\t
}
"""
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

var defaultLiveCode = `
import = [ "strings", "regexp" ]

[[plugins]]
name = "basic_uncensor_checker"
define = """
prefixList := []string {

}
regList := []*regexp.Regexp{
    regexp.MustCompile("(?i)^\\\\d+[-|_]\\\\d+$"),
    regexp.MustCompile("(?i)^N\\\\d+$"),
    regexp.MustCompile("(?i)^K\\\\d+$"),
    regexp.MustCompile("(?i)^KB\\\\d+$"),
    regexp.MustCompile("(?i)^C\\\\d+-KI\\\\d+$"),
    regexp.MustCompile("(?i)^1PON.*$"),
    regexp.MustCompile("(?i)^CARIB.*$"),
    regexp.MustCompile("(?i)^SM3D2DBD.*$"),
    regexp.MustCompile("(?i)^SMDV.*$"),
    regexp.MustCompile("(?i)^SKY.*$"),
    regexp.MustCompile("(?i)^HEY.*$"),
    regexp.MustCompile("(?i)^FC2.*$"),
    regexp.MustCompile("(?i)^MKD.*$"),
    regexp.MustCompile("(?i)^MKBD.*$"),
    regexp.MustCompile("(?i)^H4610.*$"),
    regexp.MustCompile("(?i)^H0930.*$"),
    regexp.MustCompile("(?i)^MD[-|_].*$"),
    regexp.MustCompile("(?i)^SMD[-|_].*$"),
    regexp.MustCompile("(?i)^SSDV[-|_].*$"),
    regexp.MustCompile("(?i)^CCDV[-|_].*$"),
    regexp.MustCompile("(?i)^LLDV[-|_].*$"),
    regexp.MustCompile("(?i)^DRC[-|_].*$"),
    regexp.MustCompile("(?i)^MXX[-|_].*$"),
    regexp.MustCompile("(?i)^DSAM[-|_].*$"),
    regexp.MustCompile("(?i)^JVR[-|_].*$"),
    regexp.MustCompile("(?i)COSPURI[-|_].*$"),
    regexp.MustCompile("(?i)MADOU[-|_].*$"),
}     
"""
function = """
func(ctx context.Context, number string) (bool, error) {
    for _, reg := range regList {
        if reg.MatchString(number) {
            return true, nil 
        }
    }
    for _, prefix := range prefixList {
        if strings.HasPrefix(number, prefix) {
            return true, nil 
        }
    }
    return false, nil 
}
"""

`

func TestLiveUncensorChecker(t *testing.T) {
	ck, err := NewNumberUncensorChecker(defaultLiveCode)
	if err != nil {
		t.Fatal(err)
	}
	m := map[string]bool{
		"FC2-PPV-123":               true,
		"HEYZO-222":                 true,
		"1PON-12345":                true,
		"MXX-22222":                 true,
		"JVR-22222":                 true,
		"H0930-22222":               true,
		"DSAM-22222":                true,
		"CARIB-22222":               true,
		"SM3D2DBD-22222":            true,
		"SSDV-22222":                true,
		"112214_292":                true,
		"112114-291":                true,
		"n11451":                    true,
		"heyzo_1545":                true,
		"hey-1111":                  true,
		"carib-11111-222":           true,
		"22222-333":                 true,
		"010111-222":                true,
		"H4610-Ki1111":              true,
		"MKD-12345":                 true,
		"fc2-ppv-12345":             true,
		"1pon-123":                  true,
		"smd-1234":                  true,
		"kb2134":                    true,
		"c0930-ki240528":            true,
		"YMDS-164":                  false,
		"MBRBI-002":                 false,
		"LUKE-036":                  false,
		"SMDY-123":                  false,
		"COSPURI-aaa1111":           true,
		"COSPURI-RIA-RUOK-aaaa1111": true,
		"MADOU-xg-123":              true, //麻豆的部分番号跟现有的番号冲突了, 导致无法正确区分到底是骑兵还是步兵, so, 直接给麻豆的视频都加个mdq前缀吧, 懒得处理了...
		"MADOU-cm-123":              true,
		"MADOU-md-123":              true,
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
