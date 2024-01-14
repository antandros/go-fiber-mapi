package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gofiber/contrib/fiberzap/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/monitor"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

type EndPoint struct {
	function      func(*fiber.Ctx) error
	requestbody   interface{}
	responseModel interface{}
	QueryParams   interface{}
	Aggregate     []M
	IsPublic      bool
	IsCustom      bool
	IsPost        bool
	IsAggregade   bool
	Single        bool
	List          bool
	Name          string
	PrimaryId     string
	Tags          []string
	Description   string
	path          string
	docpath       string
}

func (end *EndPoint) SetName(name string) {
	end.Name = name
}

func (end *EndPoint) SetDescription(desc string) {
	end.Description = desc
}

type M map[string]interface{}

type ModelInterface interface {
	GetEndPoints() []*EndPoint
	PostEndPoints() []*EndPoint
	PutEndPoints() []*EndPoint
	DeleteEndPoints() []*EndPoint
	SetDebug(bool)
	Generate()
	GetModelType() interface{}
	GetName() string
	SetDb(*mongo.Database)
}
type DefaultQuery struct {
	Offset int64 `json:"offset,omitempty"`
	Limit  int64 `json:"limit,omitempty"`
}

type App struct {
	models             []ModelInterface
	dbCon              *mongo.Database
	cors               *cors.Config
	mongoClient        *mongo.Client
	authMiddleware     func(*fiber.Ctx) (M, error)
	GetEndPoints       []*EndPoint
	PostEndPoints      []*EndPoint
	PutEndPoints       []*EndPoint
	DeleteEndPoints    []*EndPoint
	conurl             string
	logPath            string
	dbName             string
	fiberApp           *fiber.App
	currentCtx         *fiber.Ctx
	errorLogger        *zap.Logger
	MongoClientOptions *options.ClientOptions
	Name               string
	Description        string
	BaseURL            []string
	SaveLog            bool
	LogLife            time.Duration
	Debug              bool
}

func (app *App) GetDb() *mongo.Database {
	return app.dbCon
}
func (app *App) CreateConnection() {
	opt := options.Client()
	if app.MongoClientOptions != nil {
		opt = app.MongoClientOptions
	}
	opt = opt.ApplyURI(app.conurl)
	client, err := mongo.Connect(context.Background(), opt)
	if err != nil {
		panic(err)
	}
	app.mongoClient = client
	app.dbCon = app.mongoClient.Database(app.dbName)
}
func (app *App) SetAuthMiddleware(fnc func(*fiber.Ctx) (M, error)) {
	app.authMiddleware = fnc
}
func (app *App) getPathName(c *fiber.Ctx) string {

	for _, rn := range c.App().GetRoutes() {

		/*isMatch := RoutePatternMatch(c.Path(), rn.Path, fiber.Config{
			CaseSensitive: false,
			StrictRouting: false,
		})*/
		if rn.Method == c.Method() {
			ptn := c.Path()
			if ptn[len(ptn)-1:] == "/" {
				ptn = ptn[:len(ptn)-1]
			}
			ptn2 := fiber.RoutePatternMatch(ptn, rn.Path)
			ptn3 := fiber.RoutePatternMatch(c.Path(), rn.Path)
			if app.Debug {
				fmt.Println("ptn", ptn, "ptn2", ptn2, "ptn3", ptn3, "path", c.Path(), "rn path", rn.Path, "rn name", rn.Name)
			}
			if ptn2 {
				return rn.Name
			}
			if ptn3 {
				return rn.Name
			}
		}
	}
	return ""
}
func (app *App) authControl(c *fiber.Ctx) error {
	elmPath := strings.ReplaceAll(c.Path(), "/api/", "")
	if elmPath[len(elmPath)-1:] != "/" {
		elmPath = fmt.Sprintf("%s/", elmPath)
	}
	knowName := app.getPathName(c)
	fmt.Println("knowName", knowName)
	if app.authMiddleware != nil {
		route := c.Route()

		var founded *EndPoint

		for i := range app.models {
			var enpoints []*EndPoint
			switch route.Method {
			case "GET":
				enpoints = app.models[i].GetEndPoints()
			case "POST":
				enpoints = app.models[i].PostEndPoints()
			case "DELETE":
				enpoints = app.models[i].DeleteEndPoints()
			case "PUT":
				enpoints = app.models[i].PutEndPoints()
			}
			for _, endpoint := range enpoints {
				if app.Debug {
					fmt.Println("Model Find", "endpoint.path", endpoint.path, "elmPath", elmPath, "endpoint", endpoint.Name, "knowName", knowName)
				}
				if strings.EqualFold(endpoint.path, elmPath) || endpoint.Name == knowName {
					founded = endpoint
					fmt.Println("is found ?")
					c.Locals("model", app.models[i])
					continue
				}
			}
			if founded != nil {
				break
			}

		}
		var enpoints []*EndPoint
		c.Append("X-REQUEST-MTH", knowName)
		if founded == nil {
			switch route.Method {
			case "GET":
				enpoints = app.GetEndPoints
			case "POST":
				enpoints = app.PostEndPoints
			case "PUT":
				fmt.Println("call put")
				enpoints = app.PutEndPoints
			}
			for i := range enpoints {
				if app.Debug {

					fmt.Println("find endpoint :", enpoints[i].Name, "knowname:", knowName)
				}
				if strings.EqualFold(enpoints[i].path, elmPath) || strings.EqualFold(enpoints[i].Name, knowName) {
					founded = enpoints[i]
					break
				}
			}
		}

		if founded != nil {
			c.Locals("endpoint", founded)
			fmt.Println("put found", founded.IsPublic)
			c.Append("X-REQUEST-FND", founded.Name)
			if founded.IsPublic {
				return c.Next()
			}

			extraQuery, err := app.authMiddleware(c)
			if err != nil {
				return c.Status(403).JSON(Response{
					Message:    "Unauthorized",
					StatusCode: 403,
					Error:      err.Error(),
				})
			}
			c.Locals("authQuery", extraQuery)
			return c.Next()
		} else {
			fmt.Println("not founded")
		}

	}
	return c.Next()
}
func (app *App) RegisterGetEndpoint(path string, isPublic bool, request interface{}, response interface{}, fnc func(*fiber.Ctx) error, tags []string) *EndPoint {
	end := new(EndPoint)
	end.path = path

	end.function = fnc
	end.responseModel = response
	end.IsCustom = true
	end.IsPublic = isPublic
	end.requestbody = request
	end.Tags = tags
	end.docpath = path
	end.Name = uuid.NewString()
	app.GetEndPoints = append(app.GetEndPoints, end)
	return end
}
func (app *App) RegisterPostEndpoint(path string, isPublic bool, request interface{}, response interface{}, fnc func(*fiber.Ctx) error) *EndPoint {
	end := new(EndPoint)
	end.path = path
	end.function = fnc
	end.IsPublic = isPublic
	end.IsPost = true
	end.IsCustom = true
	end.responseModel = response
	end.Name = uuid.NewString()
	end.docpath = path
	end.requestbody = request
	app.PostEndPoints = append(app.PostEndPoints, end)
	return end
}
func (app *App) RegisterPutEndpoint(path string, isPublic bool, request interface{}, response interface{}, fnc func(*fiber.Ctx) error) *EndPoint {
	end := new(EndPoint)
	end.path = path
	end.function = fnc
	end.IsPublic = isPublic
	end.IsCustom = true
	end.IsPost = true
	end.responseModel = response
	end.Name = uuid.NewString()
	end.docpath = path
	end.requestbody = request
	app.PostEndPoints = append(app.PostEndPoints, end)
	return end
}
func (app *App) RegisterModel(item ModelInterface) {

	item.Generate()
	item.SetDebug(app.Debug)
	app.models = append(app.models, item)
}

func New(con string, db string, logPath string) *App {
	app := &App{
		conurl:  con,
		dbName:  db,
		logPath: logPath,
	}
	app.errorLogger = app.GetErrorZap()
	return app
}

func (app *App) GetZap() *zap.Logger {
	filepath := filepath.Join(app.logPath, "app.log")
	file := zapcore.AddSync(&lumberjack.Logger{
		Filename:   filepath,
		MaxSize:    4, // megabytes
		MaxBackups: 3,
		MaxAge:     2, // days
	})

	level := zap.NewAtomicLevelAt(zap.InfoLevel)

	productionCfg := zap.NewProductionEncoderConfig()
	productionCfg.TimeKey = "timestamp"
	productionCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	fileEncoder := zapcore.NewJSONEncoder(productionCfg)

	core := zapcore.NewTee(
		zapcore.NewCore(fileEncoder, file, level),
	)

	return zap.New(core)
}
func (app *App) GetErrorZap() *zap.Logger {
	filepath := filepath.Join(app.logPath, "app_error.log")

	file := zapcore.AddSync(&lumberjack.Logger{
		Filename:   filepath,
		MaxSize:    4, // megabytes
		MaxBackups: 3,
		MaxAge:     2, // days
	})

	level := zap.NewAtomicLevelAt(zap.InfoLevel)

	productionCfg := zap.NewProductionEncoderConfig()
	productionCfg.TimeKey = "timestamp"
	productionCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	fileEncoder := zapcore.NewJSONEncoder(productionCfg)

	core := zapcore.NewTee(
		zapcore.NewCore(fileEncoder, file, level),
	)

	return zap.New(core)
}
func (app *App) FindCollection(collection string, query M) (*mongo.Cursor, error) {
	return app.dbCon.Collection(collection).Find(app.currentCtx.Context(), query)
}
func (app *App) GetCollection(collection string) *mongo.Collection {
	return app.dbCon.Collection(collection)
}

func (app *App) FindOneCollection(collection string, query M) *mongo.SingleResult {
	return app.dbCon.Collection(collection).FindOne(app.currentCtx.Context(), query)
}
func (app *App) AggrageteCollection(collection string, query []M, options ...*options.AggregateOptions) (*mongo.Cursor, error) {
	return app.dbCon.Collection(collection).Aggregate(app.currentCtx.Context(), query, options...)
}
func (app *App) SetCors(origins []string, headers []string) {

	app.cors = &cors.Config{}
	app.cors.AllowHeaders = strings.Join(headers, ",")
	app.cors.AllowOrigins = strings.Join(origins, ",")
}
func (app *App) LogDbInit() {
	collections, err := app.dbCon.ListCollectionNames(context.Background(), M{})
	if err != nil {
		panic(err)
	}
	created := false
	for _, item := range collections {
		if item == "fimapi_api_log" {
			created = true
		}
	}
	if !created {

		err := app.dbCon.CreateCollection(context.Background(), "fimapi_api_log")

		if err != nil {
			panic(err)
		}
		col := app.dbCon.Collection("fimapi_api_log")
		var indexes []mongo.IndexModel
		duration := int32(app.LogLife.Seconds())
		indexAfterClear := mongo.IndexModel{
			Keys: M{"date": 1},
			Options: &options.IndexOptions{
				ExpireAfterSeconds: &duration,
			},
		}
		indexes = append(indexes, indexAfterClear)
		resp, err := col.Indexes().CreateOne(context.Background(), indexAfterClear)
		fmt.Println("Index Create", resp, err)
	}
}
func (app *App) SetDb() {
	for i := range app.models {
		app.models[i].SetDb(app.dbCon)
	}
}
func (app *App) Run(host string) {
	app.CreateConnection()
	app.SetDb()
	if app.SaveLog {
		if app.LogLife.Milliseconds() == 0 {
			app.LogLife = time.Hour * 24 * 10
			app.LogDbInit()
		}
	}
	fConfig := fiber.Config{
		ReadTimeout:  time.Second * 10,
		WriteTimeout: time.Second * 10,
	}
	if !app.Debug {
		fConfig.ErrorHandler = func(ctx *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			// Retrieve the custom status code if it's a *fiber.Error
			var e *fiber.Error
			if errors.As(err, &e) {
				if e != nil {
					code = e.Code
				}
			}
			var errid string
			if eid, ok := ctx.UserContext().Value("request_id").(string); ok {
				errid = eid

			}
			return ctx.Status(code).JSON(Response{
				StatusCode: 500,
				Error:      M{"error_id": errid},
				Message:    "internal server error",
			})
		}
	}
	fapp := fiber.New(fConfig)
	fapp.Use(func(c *fiber.Ctx) error {
		app.currentCtx = c
		reqUUid := uuid.NewString()
		ctx := context.WithValue(c.UserContext(), "request_id", reqUUid)
		c.Append("X-REQUEST-ID", reqUUid)
		c.SetUserContext(ctx)
		return c.Next()
	})
	if app.cors != nil {
		fapp.Use(cors.New(*app.cors))
	}
	fapp.Use(app.authControl)
	if app.SaveLog {
		fapp.Use(func(c *fiber.Ctx) error {
			t := time.Now()

			respData := c.Next()
			duration := time.Since(t).Microseconds()
			locals := M{}
			c.Context().VisitUserValuesAll(func(i1, i2 interface{}) {

				if lData, ok := i2.(string); ok {
					if lKey, ok := i1.(string); ok {
						locals[lKey] = lData
					}
				}
			})
			logItem := ApiLog{
				Locals:          locals,
				Uri:             c.BaseURL(),
				Date:            primitive.NewDateTimeFromTime(t),
				Duration:        duration,
				OriginalUri:     c.OriginalURL(),
				ResponseCode:    c.Response().StatusCode(),
				Method:          c.Method(),
				RequestIp:       c.IP(),
				RequestIpS:      c.IPs(),
				RawRequest:      c.Request().String(),
				RawResponse:     c.Response().String(),
				RequestHeaders:  c.GetReqHeaders(),
				ResponseHeaders: c.GetRespHeaders(),
			}
			go func(bapp *App, item ApiLog) {

				bapp.dbCon.Collection("fimapi_api_log").InsertOne(context.Background(), item)

			}(app, logItem)
			return respData
		})
	}
	fapp.Use(fiberzap.New(fiberzap.Config{
		Logger: app.GetZap(),
		FieldsFunc: func(c *fiber.Ctx) []zap.Field {
			var fields []zap.Field
			reqId := c.UserContext().Value("request_id").(string)
			fields = append(fields, zap.String("request_id", reqId))
			return fields
		},
	}))

	fapp.Get("/metrics", monitor.New())
	if !app.Debug {

		fapp.Use(recover.New(
			recover.Config{
				EnableStackTrace: true,
				StackTraceHandler: func(c *fiber.Ctx, e interface{}) {
					err := fmt.Sprintf("panic: %v\n%s\n", e, debug.Stack())
					reqId := c.UserContext().Value("request_id").(string)
					app.errorLogger.Error("Error", zap.Error(e.(error)), zap.String("request_id", reqId), zap.String("stack", err))
				},
			},
		))
	}
	NewDoc(app)

	for _, end := range app.GetEndPoints {
		if app.Debug {
			fmt.Println(end.Name, end.path)
		}
		fapp.Get(end.path, end.function).Name(end.Name)
	}

	for _, end := range app.PostEndPoints {
		fapp.Post(end.path, end.function).Name(end.Name)
	}
	for i := range app.models {
		endppintsget := app.models[i].GetEndPoints()
		endppintspost := app.models[i].PostEndPoints()
		endppintput := app.models[i].PutEndPoints()
		endppintdelete := app.models[i].DeleteEndPoints()
		for iget := range endppintdelete {
			pName := fmt.Sprintf("%d-%d-%s-%s", iget, i, endppintdelete[iget].Name, uuid.NewString())
			fapp.Delete(fmt.Sprintf("api/%s", endppintdelete[iget].path), endppintdelete[iget].function).Name(pName)
			endppintdelete[iget].Name = pName
		}
		for iget := range endppintput {
			pName := fmt.Sprintf("%d-%d-%s-%s", iget, i, endppintput[iget].Name, uuid.NewString())
			fapp.Put(fmt.Sprintf("api/%s", endppintput[iget].path), endppintput[iget].function).Name(pName)
			endppintput[iget].Name = pName
		}
		for iget := range endppintsget {
			pName := fmt.Sprintf("%d-%d-%s-%s", iget, i, endppintsget[iget].Name, uuid.NewString())
			fapp.Get(fmt.Sprintf("api/%s", endppintsget[iget].path), endppintsget[iget].function).Name(pName)
			endppintsget[iget].Name = pName
		}
		for iget := range endppintspost {
			pName := fmt.Sprintf("%d-%d-%s-%s", iget, i, endppintspost[iget].Name, uuid.NewString())

			fapp.Post(fmt.Sprintf("api/%s", endppintspost[iget].path), endppintspost[iget].function).Name(pName)
			endppintspost[iget].Name = pName
		}
	}
	app.fiberApp = fapp
	if app.Debug {
		data, _ := json.MarshalIndent(fapp.Stack(), "", "  ")
		fmt.Println(string(data))

	}
	fapp.Listen(host)
}
