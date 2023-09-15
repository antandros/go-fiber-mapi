package app

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gofiber/contrib/fiberzap/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/monitor"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/google/uuid"
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
	Aggregate     []M
	IsPublic      bool
	IsCustom      bool
	IsPost        bool
	IsAggregade   bool
	Single        bool
	List          bool
	Name          string
	Description   string
	path          string
	docpath       string
}
type M map[string]interface{}

type ModelInterface interface {
	GetEndPoints() []*EndPoint
	PostEndPoints() []*EndPoint
	PutEndPoints() []*EndPoint
	DeleteEndPoints() []*EndPoint
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
	models         []ModelInterface
	dbCon          *mongo.Database
	mongoClient    *mongo.Client
	authMiddleware func(*fiber.Ctx) (M, error)
	GetEndPoints   []*EndPoint
	PostEndPoints  []*EndPoint
	conurl         string
	logPath        string
	dbName         string
	fiberApp       *fiber.App
	currentCtx     *fiber.Ctx
	errorLogger    *zap.Logger
	Name           string
	Description    string
	BaseURL        string
	Debug          bool
}

func (app *App) CreateConnection() {
	opt := options.Client()
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

		ptn := c.Path()
		if ptn[len(ptn)-1:] == "/" {
			ptn = ptn[:len(ptn)-1]
		}
		ptn2 := fiber.RoutePatternMatch(ptn, rn.Path)
		if ptn2 {
			return rn.Name
		}
	}
	return ""
}
func (app *App) authControl(c *fiber.Ctx) error {
	elmPath := strings.ReplaceAll(c.OriginalURL(), "/api/", "")
	knowName := app.getPathName(c)
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
			}
			for _, endpoint := range enpoints {
				if strings.EqualFold(endpoint.path, elmPath) || endpoint.Name == knowName {
					founded = endpoint
					c.Locals("model", app.models[i])
					break
				}
			}

		}
		var enpoints []*EndPoint
		c.Append("X-REQUEST-ID", knowName)
		switch route.Method {
		case "GET":
			enpoints = app.GetEndPoints
		case "POST":
			enpoints = app.PostEndPoints
		}
		for i := range enpoints {
			if strings.EqualFold(enpoints[i].path, elmPath) || enpoints[i].Name == knowName {
				founded = enpoints[i]
				break
			}
		}

		if founded.IsPublic {
			return c.Next()
		}
		c.Locals("endpoint", founded)
		extraQuery, err := app.authMiddleware(c)
		if err != nil {
			return c.Status(401).JSON(Response{
				Message:    "Unauthorized",
				StatusCode: 401,
				Error:      err.Error(),
			})
		}
		c.Locals("authQuery", extraQuery)
		return c.Next()
	}
	return c.Next()
}
func (app *App) RegisterGetEndpoint(path string, isPublic bool, request interface{}, response interface{}, fnc func(*fiber.Ctx) error) *EndPoint {
	end := new(EndPoint)
	end.path = path
	end.function = fnc
	end.responseModel = response
	end.IsPublic = isPublic
	end.requestbody = request
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
	end.responseModel = response
	end.Name = uuid.NewString()
	end.docpath = path
	end.requestbody = request
	app.PostEndPoints = append(app.PostEndPoints, end)
	return end
}
func (app *App) RegisterModel(item ModelInterface) {
	item.SetDb(app.dbCon)
	item.Generate()
	app.models = append(app.models, item)
}

func New(con string, db string, logPath string) *App {
	app := &App{
		conurl:  con,
		dbName:  db,
		logPath: logPath,
	}
	app.errorLogger = app.GetErrorZap()
	app.CreateConnection()
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

func (app *App) FindOneCollection(collection string, query M) *mongo.SingleResult {
	return app.dbCon.Collection(collection).FindOne(app.currentCtx.Context(), query)
}
func (app *App) AggrageteCollection(collection string, query []M) (*mongo.Cursor, error) {
	return app.dbCon.Collection(collection).Aggregate(app.currentCtx.Context(), query)
}
func (app *App) Run(host string) {
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
	fapp.Use(fiberzap.New(fiberzap.Config{
		Logger: app.GetZap(),
		FieldsFunc: func(c *fiber.Ctx) []zap.Field {
			var fields []zap.Field
			reqId := c.UserContext().Value("request_id").(string)
			fields = append(fields, zap.String("request_id", reqId))
			return fields
		},
	}))
	fapp.Use(func(c *fiber.Ctx) error {
		app.currentCtx = c
		reqUUid := uuid.NewString()
		ctx := context.WithValue(c.UserContext(), "request_id", reqUUid)
		c.Append("X-REQUEST-ID", reqUUid)
		c.SetUserContext(ctx)
		return c.Next()
	})
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
	fapp.Use(app.authControl)
	for _, end := range app.GetEndPoints {
		fapp.Get(end.path, end.function).Name(end.Name)
	}

	for _, end := range app.PostEndPoints {
		fapp.Post(end.path, end.function).Name(end.Name)
	}
	for i := range app.models {
		endppints := app.models[i].GetEndPoints()
		endppintspost := app.models[i].PostEndPoints()
		endppintput := app.models[i].PutEndPoints()
		endppintdelete := app.models[i].DeleteEndPoints()
		for iget := range endppintdelete {
			fapp.Delete(fmt.Sprintf("api/%s", endppints[iget].path), endppints[iget].function).Name(endppints[iget].Name)
		}
		for iget := range endppintput {
			fapp.Put(fmt.Sprintf("api/%s", endppints[iget].path), endppints[iget].function).Name(endppints[iget].Name)
		}
		for iget := range endppints {
			fapp.Get(fmt.Sprintf("api/%s", endppints[iget].path), endppints[iget].function).Name(endppints[iget].Name)
		}
		for iget := range endppintspost {
			fapp.Post(fmt.Sprintf("api/%s", endppintspost[iget].path), endppintspost[iget].function).Name(endppints[iget].Name)
		}
	}
	app.fiberApp = fapp
	fapp.Listen(host)
}
