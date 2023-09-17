package app

import "go.mongodb.org/mongo-driver/bson/primitive"

type ApiLog struct {
	Locals          M
	Date            primitive.DateTime
	Duration        int64
	Uri             string
	OriginalUri     string
	ResponseCode    int
	Method          string
	RequestIp       string
	RequestIpS      []string
	RawRequest      string
	RawResponse     string
	RequestHeaders  map[string]string
	ResponseHeaders map[string]string
}
