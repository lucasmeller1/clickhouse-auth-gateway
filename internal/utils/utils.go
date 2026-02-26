package utils

import (
	"context"
	"errors"
)

func BytesToMiB(length int) float64 {
	return float64(length) / (1024 * 1024)
}

func IsCanceled(ctx context.Context, err error) bool {
	return errors.Is(err, context.Canceled) || ctx.Err() != nil
}
