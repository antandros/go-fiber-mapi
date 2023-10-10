package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"reflect"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stoewer/go-strcase"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type ModelItem[model any] struct {
	InsertAfter            func(model, *fiber.Ctx)
	InsertBefore           func(model, *fiber.Ctx) model
	SaveFunction           func(model, *fiber.Ctx)
	DeleteFunction         func(model, *fiber.Ctx)
	GetFunction            func(model, *fiber.Ctx)
	ListFunction           func(model, *fiber.Ctx)
	AuthMiddleware         func(*fiber.Ctx) (M, error)
	UpdateFunction         func(model, *fiber.Ctx)
	model                  interface{}
	modelIt                interface{}
	AppendQuery            M
	indexes                M
	NoInsert               bool
	NoDelete               bool
	IsPublic               bool
	NoUpdate               bool
	LimitNoChange          bool
	Debug                  bool
	SoftDelete             bool
	NoGet                  bool
	responseLimit          int64
	NoList                 bool
	collection             string
	Title                  string
	Description            string
	DescTags               []string
	QueryParams            interface{}
	modelType              reflect.Type
	UpdateOnAddFunction    func(item M, c *fiber.Ctx) (M, error)
	UpdateOnUpdateFunction func(item M, c *fiber.Ctx) (M, error)
	endpointsGet           []*EndPoint
	endpointsPost          []*EndPoint
	endpointsDelete        []*EndPoint
	endpointsPut           []*EndPoint
	name                   string
	dbCon                  *mongo.Database
	colDb                  *mongo.Collection
}

func (mi *ModelItem[model]) AddGetEndpoint(path string, requestParams interface{}, responseModel interface{}, function func(*fiber.Ctx)) {

}

type Response struct {
	Message    string `json:"message,omitempty"`
	StatusCode int    `json:"status_code,omitempty"`
	Result     any    `json:"result,omitempty"`
	Error      any    `json:"error,omitempty"`
	Status     bool   `json:"status,omitempty"`
}

func (mi *ModelItem[model]) SetDebug(d bool) {
	mi.Debug = d
}
func (mi *ModelItem[model]) R400(c *fiber.Ctx, message string, data any) error {
	return mi.RError(c, 400, message, data)
}
func (mi *ModelItem[model]) RError(c *fiber.Ctx, code int, message string, data any) error {
	return c.Status(code).JSON(Response{
		Message:    message,
		Status:     false,
		StatusCode: code,
		Error:      data,
	})
}
func (mi *ModelItem[model]) R500(c *fiber.Ctx, message string, data any) error {
	return mi.RError(c, 500, message, data)
}
func (mi *ModelItem[model]) R404(c *fiber.Ctx, message string) error {
	return mi.RError(c, 404, message, nil)
}
func (mi *ModelItem[model]) ROk(c *fiber.Ctx, code int, message string, data any) error {
	return c.Status(code).JSON(Response{
		Message:    message,
		Status:     true,
		StatusCode: code,
		Result:     data,
	})
}
func (mi *ModelItem[model]) R200(c *fiber.Ctx, message string, data any) error {
	return mi.ROk(c, 200, message, data)
}
func (mi *ModelItem[model]) R201(c *fiber.Ctx, message string, data any) error {
	return mi.ROk(c, 201, message, data)
}

func (mi *ModelItem[model]) SetInfo(title string, desc string, tags []string) {
	mi.Title = title
	mi.Description = desc
	mi.DescTags = tags
}
func (mi *ModelItem[model]) GetAggregate(c *fiber.Ctx, aggrage []M, requestItem interface{}, responseItem interface{}, method string) error {

	localGet := c.Locals("authQuery")
	var extraQuery M
	if localGet != nil {
		extraQuery = localGet.(M)
	}
	var aggrageBase []M

	aggrageBase = append(aggrageBase, aggrage...)
	//data["agg"] = aggrageBase

	resp, err := json.Marshal(aggrageBase)
	if err != nil {
		panic(err)
	}
	templateItem := template.New("base")
	tmpl, err := templateItem.Parse(string(resp))
	if err != nil {
		panic(err)
	}
	reqItemType := reflect.TypeOf(requestItem)
	reqItem := reflect.New(reqItemType).Elem().Addr().Interface()
	if strings.EqualFold(method, "get") {
		err = c.QueryParser(reqItem)
		if err != nil {
			panic(err)
		}
	} else {
		err = c.BodyParser(reqItem)
		if err != nil {
			panic(err)
		}
	}
	var tempOut bytes.Buffer
	err = tmpl.Execute(&tempOut, reqItem)
	if err != nil {
		panic(err)
	}
	err = json.Unmarshal(tempOut.Bytes(), &aggrageBase)
	if err != nil {
		panic(err)
	}
	if extraQuery != nil {
		aggrageBase = append([]M{M{"$match": extraQuery}}, aggrageBase...)
	}
	cursor, err := mi.colDb.Aggregate(c.Context(), aggrageBase)
	if err != nil {
		panic(err)
	}
	respItemType := reflect.TypeOf(responseItem)
	totalLength := cursor.RemainingBatchLength()
	sliceElem := reflect.SliceOf(respItemType)
	respItems := reflect.MakeSlice(sliceElem, totalLength, totalLength).Interface()
	err = cursor.All(c.Context(), &respItems)
	if err != nil {
		panic(err)
	}
	return mi.R200(c, "", respItems)
}

func (mi *ModelItem[model]) UpdateOnAdd(fnc func(item M, c *fiber.Ctx) (M, error)) {
	mi.UpdateOnAddFunction = fnc
}
func (mi *ModelItem[model]) UpdateOnUpdate(fnc func(item M, c *fiber.Ctx) (M, error)) {
	mi.UpdateOnAddFunction = fnc
}
func (mi *ModelItem[model]) AddAggrageEndPoint(path string, method string, responseModel interface{}, requestModel interface{}, aggrage []M) *EndPoint {

	var newAgg []M
	if mi.SoftDelete {
		newAgg = append(newAgg, M{
			"$match": M{
				"is_deleted": false,
			},
		})
	}
	newAgg = append(newAgg, aggrage...)

	e := &EndPoint{
		function: func(c *fiber.Ctx) error {
			return mi.GetAggregate(c, newAgg, requestModel, responseModel, method)
		},
		IsAggregade:   true,
		Name:          mi.name,
		docpath:       "/api/" + path,
		requestbody:   requestModel,
		responseModel: responseModel,
		Single:        true,
		path:          path,
	}
	if strings.EqualFold(method, "get") {

		mi.endpointsGet = append(mi.endpointsGet, e)
	} else {
		mi.endpointsPost = append(mi.endpointsPost, e)
	}
	return e
}
func (mi *ModelItem[model]) SetResponseLimit(limit int64) {
	mi.responseLimit = limit
}

func (mi *ModelItem[model]) GetEndPoints() []*EndPoint {
	return mi.endpointsGet
}
func (mi *ModelItem[model]) PostEndPoints() []*EndPoint {
	return mi.endpointsPost
}
func (mi *ModelItem[model]) PutEndPoints() []*EndPoint {
	return mi.endpointsPut
}
func (mi *ModelItem[model]) DeleteEndPoints() []*EndPoint {
	return mi.endpointsDelete
}
func (mi *ModelItem[model]) SetDb(db *mongo.Database) {

	mi.dbCon = db
	path := strcase.SnakeCase(mi.name)
	mi.colDb = mi.dbCon.Collection(path)
}
func (mi *ModelItem[model]) GetItem(c *fiber.Ctx) error {
	oid := c.Params("id", "")
	if oid != "" {
		var err error
		objectId, err := primitive.ObjectIDFromHex(oid)
		if err != nil {
			return mi.R400(c, "objectId decode error", M{"error": err})
		}
		query := M{}
		extraQuery := c.Locals("authQuery")
		if extraQuery != nil {
			query = extraQuery.(M)
		}
		query["_id"] = objectId
		if mi.SoftDelete {
			query["is_deleted"] = false

		}
		item := mi.colDb.FindOne(c.Context(), query)
		if item.Err() != nil {
			return mi.R404(c, "item not found")
		}
		pnm := mi.model.(reflect.Type)
		respItem := reflect.New(pnm).Elem().Addr().Interface()
		err = item.Decode(respItem)
		if err != nil {
			return mi.R500(c, "server error", err)
		}
		return mi.R200(c, "", respItem)
	}
	return mi.R400(c, "required item path", nil)
}

func (mi *ModelItem[model]) GenerateQueryParams(data map[string]string, mapData M) M {

	mType := reflect.TypeOf(mi.QueryParams)
	vType := reflect.ValueOf(mi.QueryParams)

	lenField := mType.NumField()
	for i := 0; i < lenField; i++ {
		field := mType.Field(i)
		vfield := vType.Field(i)
		fld := field.Tag
		ftag := fld.Get("field")
		jtag := fld.Get("json")

		if ftag != "-" && ftag != "" && jtag != "" {
			jName := strings.Split(jtag, ",")[0]
			if fieldData, ok := data[jName]; ok {
				mapData[ftag] = ConvertType(fieldData, vfield)
			}
		}
	}
	return mapData
}

func (mi *ModelItem[model]) GetItems(c *fiber.Ctx) error {

	query := M{}
	extraQuery := c.Locals("authQuery")
	if extraQuery != nil {
		query = extraQuery.(M)
	}
	if mi.SoftDelete {
		query["is_deleted"] = false
	}
	opt := options.Find()
	limit := int64(10)
	if mi.responseLimit > 0 {
		limit = mi.responseLimit
	}

	if mi.QueryParams != nil {

		mi.GenerateQueryParams(c.Queries(), query)
	}
	if !mi.LimitNoChange && c.QueryInt("limit", 0) != 0 {
		limit = int64(c.QueryInt("limit", 0))
	}
	offset := int64(0)
	if c.QueryInt("offset", 0) > 0 {
		offset = int64(c.QueryInt("offset", 0))
	}
	opt.SetSkip(offset)
	opt.SetLimit(limit)
	if mi.Debug {
		fmt.Println("query :", query, "offset:", offset, "limit:", limit, "collection:", mi.collection)
	}
	cursor, err := mi.colDb.Find(c.Context(), query)
	if err != nil {
		return mi.R500(c, "server error", err)
	}
	pnm := mi.model.(reflect.Type)
	totalLength := cursor.RemainingBatchLength()
	sliceElem := reflect.SliceOf(pnm)
	respItems := reflect.MakeSlice(sliceElem, totalLength, totalLength).Interface()

	err = cursor.All(c.Context(), &respItems)
	if err != nil {
		fmt.Println(err.Error())
		return mi.R500(c, "server error", err.Error())
	}

	return mi.R200(c, "", M{
		"total": totalLength,
		"items": respItems,
		"start": offset,
	})

}
func (mi *ModelItem[model]) UpdateItem(c *fiber.Ctx) error {
	pnm := mi.model.(reflect.Type)
	insertobj := reflect.New(pnm).Interface()

	err := c.BodyParser(&insertobj)

	if err != nil {
		return mi.R400(c, "body parse error", err.Error())
	}
	bb, _ := json.MarshalIndent(insertobj, "", "\t")
	var adata M
	json.Unmarshal(bb, &adata)
	if mi.SoftDelete {
		adata["is_deleted"] = false

	}
	delete(adata, "id")

	if mi.UpdateOnAddFunction != nil {
		adata, err = mi.UpdateOnAddFunction(adata, c)
		if err != nil {
			return mi.R500(c, "internal server error", err.Error())
		}
	}
	insertId, err := mi.colDb.InsertOne(c.Context(), adata)
	if err != nil {
		return mi.R500(c, "internal server error", err.Error())
	}
	objId := insertId.InsertedID.(primitive.ObjectID)
	itmCur := mi.colDb.FindOne(c.Context(), M{"_id": objId})
	if itmCur.Err() != nil {
		return mi.R500(c, "internal server error", itmCur.Err())
	}
	err = itmCur.Decode(insertobj)
	if err != nil {
		return mi.R500(c, "internal server error", err.Error())
	}
	return mi.R201(c, "item created", &insertobj)
}
func (mi *ModelItem[model]) CreateItem(c *fiber.Ctx) error {
	pnm := mi.model.(reflect.Type)
	insertobj := reflect.New(pnm).Interface()

	err := c.BodyParser(&insertobj)

	if err != nil {
		return mi.R400(c, "body parse error", err.Error())
	}
	bb, _ := json.MarshalIndent(insertobj, "", "\t")
	var adata M
	json.Unmarshal(bb, &adata)
	if mi.SoftDelete {
		adata["is_deleted"] = false

	}
	delete(adata, "id")
	if mi.UpdateOnAddFunction != nil {
		adata, err = mi.UpdateOnAddFunction(adata, c)
		if err != nil {
			return mi.R500(c, "internal server error", err.Error())
		}
	}
	insertId, err := mi.colDb.InsertOne(c.Context(), adata)
	if err != nil {
		return mi.R500(c, "internal server error", err.Error())
	}
	objId := insertId.InsertedID.(primitive.ObjectID)
	itmCur := mi.colDb.FindOne(c.Context(), M{"_id": objId})
	if itmCur.Err() != nil {
		return mi.R500(c, "internal server error", itmCur.Err())
	}
	err = itmCur.Decode(insertobj)
	if err != nil {
		return mi.R500(c, "internal server error", err.Error())
	}
	return mi.R201(c, "item created", &insertobj)
}
func (mi *ModelItem[model]) DeleteItem(c *fiber.Ctx) error {
	oid := c.Params("id", "")
	if oid != "" {
		var err error
		objectId, err := primitive.ObjectIDFromHex(oid)
		if err != nil {
			return mi.R400(c, "objectId decode error", M{"error": err})
		}
		var actionCount int
		if mi.SoftDelete {
			var result *mongo.UpdateResult
			result, err = mi.colDb.UpdateOne(c.Context(), M{"_id": objectId, "is_deleted": false}, M{"$set": M{"is_deleted": true}})
			actionCount = int(result.ModifiedCount)
		} else {
			var result *mongo.DeleteResult
			result, err = mi.colDb.DeleteOne(c.Context(), M{"_id": objectId})
			actionCount = int(result.DeletedCount)
		}

		if err != nil {
			return mi.R500(c, "server error", err)
		}
		if actionCount == 0 {
			return mi.R400(c, "item already deleted or cant found", nil)
		}
		return mi.R400(c, "required delete path", nil)
	}
	return mi.R400(c, "required delete path", nil)
}

func (mi *ModelItem[model]) GetModelType() interface{} {
	return mi.model
}
func (mi *ModelItem[model]) Tags() {
	mi.modelType = reflect.TypeOf(mi.model).Elem()
	lenField := mi.modelType.NumField()
	f := []reflect.StructField{}
	hasId := false
	hasDeleted := false
	for i := 0; i < lenField; i++ {
		fld := mi.modelType.Field(i).Tag
		hasJson := fld.Get("json")
		hasBson := fld.Get("bson")
		field := mi.modelType.Field(i)
		name := field.Name

		if hasBson == "" {
			hasBson = fmt.Sprintf("%s,omitempty", strcase.SnakeCase(name))
		}
		if hasJson == "" {
			hasJson = fmt.Sprintf("%s,omitempty", strcase.SnakeCase(name))
		}
		f = append(f, reflect.StructField{
			Name: name,
			Type: field.Type,
			Tag:  reflect.StructTag(fmt.Sprintf(`json:"%s" bson:"%s"`, hasJson, hasBson)),
		})
		if name == "Id" {
			hasId = true
		}
		if name == "IsDeleted" {
			hasDeleted = true
		}
	}
	if !hasId {
		f = append(f, reflect.StructField{
			Name: "Id",
			Type: reflect.TypeOf(primitive.ObjectID{}),
			Tag:  reflect.StructTag(`json:"id,omitempty" bson:"_id,omitempty"`),
		})
	}

	if mi.SoftDelete {
		if !hasDeleted {
			f = append(f, reflect.StructField{
				Name: "IsDeleted",
				Type: reflect.TypeOf(true),
				Tag:  reflect.StructTag(`json:"-" bson:"is_deleted"`),
			})
		}
	}
	mi.model = reflect.StructOf(f)
}
func (mi *ModelItem[model]) GetName() string {
	return mi.name
}
func (mi *ModelItem[model]) Generate() {
	mi.name = reflect.TypeOf(mi.modelIt).Elem().Name()
	path := strcase.SnakeCase(mi.name)
	if !mi.NoDelete {
		mi.endpointsDelete = append(mi.endpointsDelete, &EndPoint{
			function:      mi.DeleteItem,
			Name:          uuid.NewString(),
			responseModel: Response{},
			Single:        true,
			path:          fmt.Sprintf("%s/:id", path),
			docpath:       fmt.Sprintf("/api/%s/{id}", path),
		})
	}
	if !mi.NoGet {
		mi.endpointsGet = append(mi.endpointsGet, &EndPoint{
			function:      mi.GetItem,
			Name:          uuid.NewString(),
			Single:        true,
			responseModel: Response{},
			QueryParams:   mi.QueryParams,
			path:          fmt.Sprintf("%s/:id", path),
			docpath:       fmt.Sprintf("/api/%s/{id}", path),
		})
	}
	if !mi.NoUpdate {
		mi.endpointsPut = append(mi.endpointsPut, &EndPoint{
			function:      mi.GetItem,
			Name:          uuid.NewString(),
			Single:        true,
			responseModel: Response{},
			path:          fmt.Sprintf("%s/:id", path),
			docpath:       fmt.Sprintf("/api/%s/{id}", path),
		})
	}
	if !mi.NoList {
		mi.endpointsGet = append(mi.endpointsGet, &EndPoint{
			function:      mi.GetItems,
			Name:          uuid.NewString(),
			List:          true,
			QueryParams:   mi.QueryParams,
			responseModel: Response{},
			path:          fmt.Sprintf("%s/", path),
			docpath:       fmt.Sprintf("/api/%s/", path),
		})
	}
	if !mi.NoInsert {
		mi.endpointsPost = append(mi.endpointsPost, &EndPoint{
			function:      mi.CreateItem,
			Name:          uuid.NewString(),
			responseModel: Response{},
			Single:        true,
			path:          fmt.Sprintf("%s/", path),
			docpath:       fmt.Sprintf("/api/%s/", path),
		})
	}
}

func NewModel[Model any](collection string) *ModelItem[Model] {
	item := new(Model)

	mdl := &ModelItem[Model]{collection: collection, model: item, modelIt: item}
	mdl.Tags()
	return mdl
}
