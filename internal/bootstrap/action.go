package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"
)

var (
	errMissingDependency = errors.New("missing dependency")
	errDuplicateProvide  = errors.New("duplicate provide")
	errCyclicDependency  = errors.New("cyclic dependency detected")
)

type InitFunc func(ctx context.Context, sc *StartContext) error

type InitAction struct {
	Name     string
	Requires []string
	Provides []string
	Run      InitFunc
}

// Sort performs a topological sort on the action list based on Requires/Provides.
// Actions are reordered so that each action's required resources are provided by
// a preceding action. Returns an error on missing dependencies, duplicate provides,
// or cycles. Among actions with no ordering constraint, the original input order is preserved.
func Sort(actions []InitAction) ([]InitAction, error) {
	if len(actions) == 0 {
		return actions, nil
	}

	provider, err := buildProviderMap(actions)
	if err != nil {
		return nil, err
	}

	adj, inDeg, err := buildDepGraph(actions, provider)
	if err != nil {
		return nil, err
	}

	return topoSort(actions, adj, inDeg)
}

func buildProviderMap(actions []InitAction) (map[string]int, error) {
	provider := make(map[string]int)
	for i, a := range actions {
		for _, p := range a.Provides {
			if prev, ok := provider[p]; ok {
				return nil, fmt.Errorf(
					"action %q provides %q, which is already provided by %q: %w",
					a.Name, p, actions[prev].Name, errDuplicateProvide,
				)
			}
			provider[p] = i
		}
	}
	return provider, nil
}

func buildDepGraph(actions []InitAction, provider map[string]int) ([][]int, []int, error) {
	n := len(actions)
	adj := make([][]int, n)
	inDeg := make([]int, n)

	for i, a := range actions {
		seen := make(map[int]struct{})
		for _, req := range a.Requires {
			pi, ok := provider[req]
			if !ok {
				return nil, nil, fmt.Errorf(
					"action %q requires %q, but no action provides it: %w",
					a.Name, req, errMissingDependency,
				)
			}
			if pi == i {
				return nil, nil, fmt.Errorf(
					"action %q requires %q which it provides itself: %w",
					a.Name, req, errCyclicDependency,
				)
			}
			if _, dup := seen[pi]; dup {
				continue
			}
			seen[pi] = struct{}{}
			adj[pi] = append(adj[pi], i)
			inDeg[i]++
		}
	}
	return adj, inDeg, nil
}

func topoSort(actions []InitAction, adj [][]int, inDeg []int) ([]InitAction, error) {
	n := len(actions)
	queue := make([]int, 0, n)
	for i := range inDeg {
		if inDeg[i] == 0 {
			queue = append(queue, i)
		}
	}

	sorted := make([]InitAction, 0, n)
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		sorted = append(sorted, actions[cur])
		for _, dep := range adj[cur] {
			inDeg[dep]--
			if inDeg[dep] == 0 {
				queue = append(queue, dep)
			}
		}
	}

	if len(sorted) != n {
		return nil, fmt.Errorf("actions contain a dependency cycle: %w", errCyclicDependency)
	}
	return sorted, nil
}

// Validate checks that the action dependency graph is valid:
// no missing dependencies, no duplicate provides, no cycles.
func Validate(actions []InitAction) error {
	_, err := Sort(actions)
	return err
}

// Execute sorts the action list by dependency order, then runs each action sequentially.
func Execute(ctx context.Context, sc *StartContext, actions []InitAction) error {
	sorted, err := Sort(actions)
	if err != nil {
		return fmt.Errorf("action validation failed: %w", err)
	}
	if sc.Infra.Logger != nil {
		names := make([]string, len(sorted))
		for i, a := range sorted {
			names[i] = a.Name
		}
		sc.Infra.Logger.Info("bootstrap action execution order",
			zap.Strings("actions", names),
		)
	}
	for _, action := range sorted {
		start := time.Now()
		if err := action.Run(ctx, sc); err != nil {
			return fmt.Errorf("%s failed: %w", action.Name, err)
		}
		if sc.Infra.Logger != nil {
			sc.Infra.Logger.Debug("bootstrap action done",
				zap.String("action", action.Name),
				zap.Duration("cost", time.Since(start)),
			)
		}
	}
	return nil
}

// ProvidedResources returns the set of all resource names provided by the action list.
func ProvidedResources(actions []InitAction) []string {
	out := make([]string, 0, len(actions)*2)
	for _, a := range actions {
		out = append(out, a.Provides...)
	}
	return out
}

// RequiredResources returns the set of all resource names required by the action list.
func RequiredResources(actions []InitAction) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0, len(actions)*2)
	for _, a := range actions {
		for _, r := range a.Requires {
			if _, ok := seen[r]; !ok {
				seen[r] = struct{}{}
				out = append(out, r)
			}
		}
	}
	return out
}
