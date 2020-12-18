package tlssocks

import (
	"context"
	"io"
)

type readerCtx struct {
	ctx context.Context
	r   io.Reader
}

func (r *readerCtx) Read(p []byte) (n int, err error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}

// NewReader gets a context-aware io.Reader.
func NewReader(ctx context.Context, r io.Reader) io.Reader {
	return &readerCtx{
		ctx: ctx,
		r:   r,
	}
}
