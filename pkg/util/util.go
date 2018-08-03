package util

import (
	"encoding/hex"

	"go.uber.org/zap/zapcore"
)

func HexField(key string, val []byte) zapcore.Field {
	return zapcore.Field{Key: key, Type: zapcore.StringType, String: hex.EncodeToString(val)}
}