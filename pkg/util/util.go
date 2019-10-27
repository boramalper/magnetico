package util

import (
	"encoding/hex"
	"math"

	"go.uber.org/zap/zapcore"
)

func HexField(key string, val []byte) zapcore.Field {
	return zapcore.Field{Key: key, Type: zapcore.StringType, String: hex.EncodeToString(val)}
}

func Round(x float64) float64 {
	t := math.Trunc(x)
	if math.Abs(x-t) >= 0.5 {
		return t + math.Copysign(1, x)
	}
	return t
}
func RoundToDecimal(iFloat float64, iDecimalPlaces int) float64{
	var multiplier float64 = 10
	for i := 1; i < iDecimalPlaces ; i++{
		multiplier = multiplier * 10
	}
	return Round(iFloat*multiplier)/multiplier
}