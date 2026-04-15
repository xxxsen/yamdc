package translator

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var errTranslatorNoData = errors.New("translator return no data")

type group struct {
	ts []ITranslator
}

func (g *group) Name() string {
	names := make([]string, 0, len(g.ts))
	for _, t := range g.ts {
		names = append(names, t.Name())
	}
	return fmt.Sprintf("G:[%s]", strings.Join(names, ","))
}

func (g *group) Translate(ctx context.Context, wording, srclang, dstlang string) (string, error) {
	var retErr error
	for _, t := range g.ts {
		rs, err := t.Translate(ctx, wording, srclang, dstlang)
		if err != nil {
			retErr = fmt.Errorf("call %s for translate failed, err:%w", t.Name(), err)
			continue
		}
		if len(rs) == 0 {
			retErr = fmt.Errorf("translator:%s: %w", t.Name(), errTranslatorNoData)
			continue
		}
		return rs, nil
	}
	return "", retErr
}

func NewGroup(ts ...ITranslator) ITranslator {
	return &group{
		ts: ts,
	}
}
