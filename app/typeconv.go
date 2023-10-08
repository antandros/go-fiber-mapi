package app

import (
	"fmt"
	"reflect"
	"strconv"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func ConvertType(data string, itemType reflect.Value) any {
	var resp any
	switch vt := itemType.Interface().(type) {
	case int16:
		resp, _ = strconv.ParseInt(data, 10, 16)
	case int32:
		resp, _ = strconv.ParseInt(data, 10, 32)
	case int64:
		resp, _ = strconv.ParseInt(data, 10, 64)
	case int:
		resp, _ = strconv.Atoi(data)
	case int8:
		resp, _ = strconv.ParseInt(data, 10, 8)
	case uint16:
		resp, _ = strconv.ParseUint(data, 10, 16)
	case uint32:
		resp, _ = strconv.ParseUint(data, 10, 32)
	case uint64:
		resp, _ = strconv.ParseUint(data, 10, 64)
	case uint:
		resp, _ = strconv.ParseUint(data, 10, 64)
	case uint8:
		resp, _ = strconv.Atoi(data)
	case float32:
		resp, _ = strconv.ParseFloat(data, 32)
	case float64:
		resp, _ = strconv.ParseFloat(data, 64)
	case primitive.ObjectID:
		resp, _ = primitive.ObjectIDFromHex(data)
	case primitive.Decimal128:
		resp, _ = primitive.ParseDecimal128(data)
	case primitive.DateTime:
		date, _ := time.Parse(time.RFC3339, data)
		resp = primitive.NewDateTimeFromTime(date)
	case bool:
		resp, _ = strconv.ParseBool(data)
	case string:
		resp = data
	default:
		fmt.Println("default", vt)
	}
	return resp
}
