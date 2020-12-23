package util

import (
	"encoding/hex"
	"math"

	"go.uber.org/zap/zapcore"
)

func HexField(key string, val []byte) zapcore.Field {
	return zapcore.Field{Key: key, Type: zapcore.StringType, String: hex.EncodeToString(val)}
}

//round iFloat to iDecimalPlaces decimal points
func RoundToDecimal(iFloat float64, iDecimalPlaces int) float64 {
	var multiplier float64 = 10
	for i := 1; i < iDecimalPlaces; i++ {
		multiplier = multiplier * 10
	}
	return math.Round(iFloat*multiplier) / multiplier
}
