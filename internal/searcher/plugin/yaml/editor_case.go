package yaml

import (
	"context"
	"fmt"
	"strings"

	"github.com/xxxsen/yamdc/internal/client"
)

func DebugCase(ctx context.Context, cli client.IHTTPClient, raw *PluginSpec, spec CaseSpec) (*CaseDebugResult, error) {
	scrape, err := DebugScrape(ctx, cli, raw, spec.Input)
	if err != nil {
		if strings.EqualFold(strings.TrimSpace(spec.Output.Status), "error") {
			return &CaseDebugResult{Pass: true}, nil
		}
		return &CaseDebugResult{Pass: false, Errmsg: err.Error()}, nil
	}
	status := "not_found"
	if scrape.Meta != nil {
		status = "success"
	}
	if expected := strings.TrimSpace(spec.Output.Status); expected != "" && !strings.EqualFold(expected, status) {
		return &CaseDebugResult{Pass: false, Errmsg: fmt.Sprintf(
			"expected status=%s but got %s",
			expected,
			status,
		), Meta: scrape.Meta}, nil
	}
	if expected := strings.TrimSpace(spec.Output.Title); expected != "" {
		got := ""
		if scrape.Meta != nil {
			got = strings.TrimSpace(scrape.Meta.Title)
		}
		if got != expected {
			return &CaseDebugResult{Pass: false, Errmsg: fmt.Sprintf(
				"expected title=%s but got %s",
				expected,
				got,
			), Meta: scrape.Meta}, nil
		}
	}
	if len(spec.Output.TagSet) != 0 {
		got := []string(nil)
		if scrape.Meta != nil {
			got = scrape.Meta.Genres
		}
		if !equalNormalizedSet(spec.Output.TagSet, got) {
			return &CaseDebugResult{Pass: false, Errmsg: fmt.Sprintf(
				"expected tag_set=%v but got %v",
				normalizeStringSet(spec.Output.TagSet),
				normalizeStringSet(got),
			), Meta: scrape.Meta}, nil
		}
	}
	if len(spec.Output.ActorSet) != 0 {
		got := []string(nil)
		if scrape.Meta != nil {
			got = scrape.Meta.Actors
		}
		if !equalNormalizedSet(spec.Output.ActorSet, got) {
			return &CaseDebugResult{Pass: false, Errmsg: fmt.Sprintf(
				"expected actor_set=%v but got %v",
				normalizeStringSet(spec.Output.ActorSet),
				normalizeStringSet(got),
			), Meta: scrape.Meta}, nil
		}
	}
	return &CaseDebugResult{Pass: true, Meta: scrape.Meta}, nil
}
