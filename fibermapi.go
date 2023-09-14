package gofibermapi

import "github.com/antandros/go-fiber-mapi/app"

func NewApp(mongoUri string, dbName string, logPath string) *app.App {
	return app.New(mongoUri, dbName, logPath)
}
