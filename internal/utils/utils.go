package utils

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var validIdentifierRegex = regexp.MustCompile(`^[a-zA-Z0-9_]{1,255}$`)

func IsValidIdentifier(s string) bool {
	return validIdentifierRegex.MatchString(s)
}

func BytesToMiB(length int) float64 {
	return float64(length) / (1024 * 1024)
}

func IsCanceled(ctx context.Context, err error) bool {
	return errors.Is(err, context.Canceled) || ctx.Err() != nil
}

func QuoteIdentifier(identifier string) string {
	escaped := strings.ReplaceAll(identifier, `"`, `""`)
	return fmt.Sprintf(`"%s"`, escaped)
}
