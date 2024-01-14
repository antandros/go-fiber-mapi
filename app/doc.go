package app

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gosimple/slug"
)

type DocInfo struct {
	Version string `json:"version"`
	Title   string `json:"title"`
	License struct {
		Name string `json:"name"`
	} `json:"license"`
}
type DocParameter struct {
	Name        string `json:"name"`
	In          string `json:"in"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Schema      struct {
		Type    string `json:"type,omitempty"`
		Maximum int    `json:"maximum,omitempty"`
		Format  string `json:"format,omitempty"`
	} `json:"schema,omitempty"`
}
type DocContent struct {
	ApplicationJSON struct {
		Schema struct {
			Ref string `json:"$ref"`
		} `json:"schema"`
	} `json:"application/json"`
}

type DocHeader struct {
	Description string `json:"description"`
	Schema      struct {
		Type string `json:"type"`
	} `json:"schema"`
}
type DocResponse struct {
	Description string               `json:"description,omitempty"`
	Headers     map[string]DocHeader `json:"headers,omitempty"`
	Content     M                    `json:"content"`
}
type PathDoc struct {
	Summary     string   `json:"summary"`
	OperationID string   `json:"operationId"`
	Tags        []string `json:"tags"`
}
type DocMethodInfo struct {
	Summary     string                 `json:"summary,omitempty"`
	OperationID string                 `json:"operationId,omitempty"`
	Tags        []string               `json:"tags,omitempty"`
	Parameters  []*DocParameter        `json:"parameters,omitempty"`
	Responses   map[string]DocResponse `json:"responses,omitempty"`
	RequestBody *DocResponse           `json:"requestBody,omitempty"`
	Security    []M                    `json:"security,omitempty"`
}
type DocEndPoint struct {
	Put    *DocMethodInfo `json:"put,omitempty"`
	Get    *DocMethodInfo `json:"get,omitempty"`
	Post   *DocMethodInfo `json:"post,omitempty"`
	Delete *DocMethodInfo `json:"delete,omitempty"`
}

type GenerateDoc struct {
	app     *App
	schemas M
	paths   M
	data    M
}

func (gd *GenerateDoc) DocGenFieldData(mType reflect.Type, inRequest bool) M {
	mapData := M{}
	if mType.Kind() == reflect.Ptr {
		mType = mType.Elem()
	}
	lenField := mType.NumField()
	for i := 0; i < lenField; i++ {
		field := mType.Field(i)
		fld := field.Tag
		jtag := fld.Get("json")
		required := fld.Get("required")
		if jtag != "-" && jtag != "" {
			nname := strings.Split(jtag, ",")[0]
			typeText := ""
			typeFormat := ""
			typeRef := ""
			if len(field.Type.Name()) > 2 {
				switch strings.ToLower(field.Type.Name()[:3]) {
				case "int":
					typeText = "integer"
					typeFormat = field.Type.Name()
				case "uin":
					typeText = "integer"
					typeFormat = field.Type.Name()
				case "tim":
					typeText = "string"
					typeFormat = "isodatetime"
				case "flo":
					typeText = "number"
					typeFormat = field.Type.Name()
				case "boo":
					typeText = "boolean"
					typeFormat = field.Type.Name()

				default:
					typeText = "string"

					if field.Type.Kind() == reflect.Struct {

						key := fmt.Sprintf("%sModel", field.Type.Name())
						pnm := field.Type

						if _, ok := gd.schemas[key]; !ok {
							docData := gd.DocGenFieldData(pnm, inRequest)
							gd.schemas[key] = M{
								"type":       "object",
								"properties": docData,
							}
							if inRequest {
								gd.schemas[key].(M)["required"] = required != ""
							}
						}

						typeRef = fmt.Sprintf("#/components/schemas/%s", key)
						typeFormat = key
						typeText = "object"
					} else if field.Type.Kind() == reflect.String {
						typeText = "string"

					} else if field.Type.Kind() == reflect.Slice || field.Type.Kind() == reflect.Array {
						if field.Type.Elem().Kind() == reflect.Struct {
							key := fmt.Sprintf("%sModel", field.Type.Name())
							pnm := field.Type

							if _, ok := gd.schemas[key]; !ok {
								docData := gd.DocGenFieldData(pnm, inRequest)
								gd.schemas[key] = M{
									"type":       "object",
									"properties": docData,
								}
								if inRequest {
									gd.schemas[key].(M)["required"] = required != ""
								}
							}

							typeRef = fmt.Sprintf("#/components/schemas/%s", key)
							typeText = "array"
						}
					} else {

						typeFormat = field.Type.Name()

					}

				}

			} else if field.Type.Kind() == reflect.Slice || field.Type.Kind() == reflect.Array {
				if field.Type.Elem().Kind() == reflect.Struct {
					pnm := field.Type.Elem()
					key := fmt.Sprintf("%sModel", pnm.Name())

					if _, ok := gd.schemas[key]; !ok {
						docData := gd.DocGenFieldData(pnm, inRequest)
						gd.schemas[key] = M{
							"type":       "object",
							"properties": docData,
						}
					}

					typeRef = fmt.Sprintf("#/components/schemas/%s", key)
					typeText = "array"
				}
			}
			if field.Type.Kind() == reflect.Slice && typeText == "" {
				typeText = "array"
			}
			if typeText == "" && field.Type.Kind().String() == "map" {
				typeText = "object"
			}
			mapData[nname] = M{
				"type":   typeText,
				"format": typeFormat,
			}
			if typeText == "array" {
				mapData[nname].(M)["items"] = M{
					"$ref": typeRef,
				}
				mapData[nname].(M)["format"] = "array"
			} else {
				if typeRef != "" {
					mapData[nname].(M)["$ref"] = typeRef

					if inRequest {
						mapData[nname].(M)["required"] = required != ""
					}

				}
			}

		}

	}
	return mapData
}
func (gd *GenerateDoc) DocTags(mi ModelInterface) M {
	item := mi.GetModelType()
	pnm := item.(reflect.Type)
	respItem := reflect.New(pnm).Elem().Addr().Interface()
	mType := reflect.TypeOf(respItem).Elem()

	return gd.DocGenFieldData(mType, false)
}
func (gd *GenerateDoc) DocTagsCustom(m any, inRequest bool) M {
	mType := reflect.TypeOf(m)

	return gd.DocGenFieldData(mType, inRequest)
}

func (gd *GenerateDoc) GenerateDocItem(model ModelInterface, endpoint *EndPoint, isPost bool, isPut bool, isDelete bool) {
	if endpoint.docpath == "" {
		return
	}
	var summary string
	var parameters []*DocParameter
	responseBase := M{}
	ref := fmt.Sprintf("#/components/schemas/%s", model.GetName())

	if endpoint.Single {
		if isPut {
			summary = fmt.Sprintf("Update a %s", model.GetName())
		} else if isDelete {
			summary = fmt.Sprintf("Delete a %s", model.GetName())
		} else {
			summary = fmt.Sprintf("Returns a single %s", model.GetName())
		}

		if !isPost {
			parameters = append(parameters, &DocParameter{
				Name:     endpoint.PrimaryId,
				In:       "path",
				Required: true,
				Schema: struct {
					Type    string "json:\"type,omitempty\""
					Maximum int    "json:\"maximum,omitempty\""
					Format  string "json:\"format,omitempty\""
				}{
					Type:    "string",
					Maximum: 24,
				},
				Description: "The id needs for fetching",
			})
		}

		responseBase = M{
			"type": "object",
			"$ref": ref,
		}
	}
	if endpoint.List {
		summary = fmt.Sprintf("Returns a all %s", model.GetName())
		var data M
		if endpoint.QueryParams != nil {
			data = gd.DocTagsCustom(endpoint.QueryParams, true)
		} else {

			data = gd.DocTagsCustom(DefaultQuery{}, true)
		}
		for key, val := range data {
			isReq := false
			if val.(M)["required"] != nil {
				isReq = val.(M)["required"].(bool)
			}
			parameters = append(parameters, &DocParameter{
				Name:     key,
				In:       "query",
				Required: isReq,
				Schema: struct {
					Type    string "json:\"type,omitempty\""
					Maximum int    "json:\"maximum,omitempty\""
					Format  string "json:\"format,omitempty\""
				}{
					Type: val.(M)["type"].(string),
				},
			})
		}
		responseBase = M{
			"type": "array",
			"items": M{
				"$ref": ref,
			},
		}
	}
	if _, ok := gd.schemas[model.GetName()]; !ok {
		gd.schemas[model.GetName()] = M{
			"type":       "object",
			"properties": gd.DocTags(model),
		}
	}
	if endpoint.IsAggregade {
		if endpoint.Description != "" {
			summary = endpoint.Description
		} else {
			summary = fmt.Sprintf("%s Aggregate", model.GetName())
		}
		parameters = parameters[:0]
		if endpoint.requestbody != nil {
			data := gd.DocTagsCustom(endpoint.requestbody, true)
			for key, val := range data {
				parameters = append(parameters, &DocParameter{
					Name:     key,
					In:       "query",
					Required: false,
					Schema: struct {
						Type    string "json:\"type,omitempty\""
						Maximum int    "json:\"maximum,omitempty\""
						Format  string "json:\"format,omitempty\""
					}{
						Type: val.(M)["type"].(string),
					},
				})
			}
		}
		if endpoint.responseModel != nil {
			kk := slug.Make(endpoint.Name)
			if _, ok := gd.schemas[kk]; ok {
				kk = fmt.Sprintf("%s%s", kk, GenerateString(6))
			}
			ref := fmt.Sprintf("#/components/schemas/%s", kk)
			gd.schemas[kk] = M{
				"type":       "object",
				"properties": gd.DocTagsCustom(endpoint.responseModel, false),
			}
			responseBase = M{
				"type": "array",
				"items": M{
					"$ref": ref,
				},
			}
		}
	}

	returnSchema := DocResponse{
		Description: "Response",
		Content: M{"application/json": M{
			"schema": M{
				"type": "object",
				"properties": M{
					"message": M{
						"type": "string",
					},
					"status_code": M{
						"type": "integer",
					},
					"status": M{
						"type": "boolean",
					},
					"result": responseBase,
				},
			},
		}},
	}
	if endpoint.List {
		returnSchema = DocResponse{
			Description: "Response",
			Content: M{"application/json": M{
				"schema": M{
					"type": "object",
					"properties": M{
						"message": M{
							"type": "string",
						},
						"status_code": M{
							"type": "integer",
						},
						"status": M{
							"type": "boolean",
						},
						"result": M{
							"type": "object",
							"properties": M{
								"start": M{
									"type": "integer",
								},
								"total": M{
									"type": "integer",
								},
								"items": responseBase,
							},
						},
					},
				},
			}},
		}

	}
	notFoundResponse := DocResponse{
		Description: "item not found",
		Content: M{"application/json": M{
			"schema": M{
				"$ref": "#/components/schemas/NotFound",
			},
		}},
	}
	internalServerError := DocResponse{
		Description: "internal server error",
		Content: M{"application/json": M{
			"schema": M{
				"$ref": "#/components/schemas/ServerError",
			},
		}},
	}
	unauthorizedResponse := DocResponse{
		Description: "Unauthorized",
		Content: M{"application/json": M{
			"schema": M{
				"$ref": "#/components/schemas/Unauthorized",
			},
		}},
	}
	resp := map[string]DocResponse{
		"404": notFoundResponse,
		"500": internalServerError,
	}
	if !endpoint.IsPublic {
		resp["401"] = unauthorizedResponse
	}

	if isPost {
		resp["201"] = returnSchema
	} else {
		resp["200"] = returnSchema
	}
	tags := []string{model.GetName()}
	if endpoint.List {
		tags = append(tags, "List Items")
	} else if endpoint.Single {
		if isDelete {
			tags = append(tags, "Delete Item")
		}
		if isPut {
			tags = append(tags, "Update Item")
		}
		if isPost {
			tags = append(tags, "Create Item")
		}
	}
	var sec []M
	if !endpoint.IsPublic {
		sec = []M{
			M{"bearerAuth": []M{}},
		}
	}
	method := &DocMethodInfo{
		Summary:    summary,
		Parameters: parameters,
		Responses:  resp,
		Tags:       tags,
		Security:   sec,
	}
	if doc, ok := gd.paths[endpoint.docpath].(DocEndPoint); ok {
		if isPost || isPut {
			text := fmt.Sprintf("Create a new a %s", model.GetName())
			if isPut {
				text = fmt.Sprintf("Update a %s", model.GetName())
			}
			method.RequestBody = &DocResponse{
				Description: text,
				Content: M{"application/json": M{
					"schema": M{
						"$ref": ref,
					},
				}},
			}
			if isPost {
				doc.Post = method
			} else {
				doc.Put = method
			}

		} else if isDelete {
			doc.Delete = method
		} else {
			doc.Get = method
		}
		gd.paths[endpoint.docpath] = doc
	} else {
		var endpointItem DocEndPoint
		if isPost {
			endpointItem = DocEndPoint{
				Post: method,
			}
		} else if isDelete {
			endpointItem = DocEndPoint{
				Delete: method,
			}
		} else if isPut {
			endpointItem = DocEndPoint{
				Put: method,
			}
		} else {
			endpointItem = DocEndPoint{
				Get: method,
			}
		}
		gd.paths[endpoint.docpath] = endpointItem
	}
}
func (gd *GenerateDoc) GenerateOtherEndpoints() {
	allEndpoints := gd.app.GetEndPoints
	allEndpoints = append(allEndpoints, gd.app.PostEndPoints...)
	notFoundResponse := DocResponse{
		Description: "item not found",
		Content: M{"application/json": M{
			"schema": M{
				"$ref": "#/components/schemas/NotFound",
			},
		}},
	}
	internalServerError := DocResponse{
		Description: "internal server error",
		Content: M{"application/json": M{
			"schema": M{
				"$ref": "#/components/schemas/ServerError",
			},
		}},
	}
	unauthorizedResponse := DocResponse{
		Description: "Unauthorized",
		Content: M{"application/json": M{
			"schema": M{
				"$ref": "#/components/schemas/Unauthorized",
			},
		}},
	}
	for _, endpoint := range allEndpoints {
		summary := fmt.Sprintf("Returns a single %s", endpoint.Name)
		if endpoint.Description != "" {
			summary = endpoint.Description
		}
		var parameters []*DocParameter
		var tags []string
		resp := map[string]DocResponse{
			"404": notFoundResponse,
			"500": internalServerError,
		}
		var sec []M

		if !endpoint.IsPublic {
			sec = []M{
				M{"bearerAuth": []M{}},
			}
		}
		if !endpoint.IsPublic {
			resp["401"] = unauthorizedResponse
		}
		kk := slug.Make(endpoint.Name)
		if len(endpoint.Name) == 36 {
			kk = slug.Make(endpoint.path)
		}
		if endpoint.responseModel != nil {

			key := fmt.Sprintf("Response%s", kk)
			gd.schemas[key] = M{
				"type":       "object",
				"properties": gd.DocTagsCustom(endpoint.responseModel, false),
			}
			ref := fmt.Sprintf("#/components/schemas/%s", key)

			resp["200"] = DocResponse{
				Description: fmt.Sprintf("%s model", endpoint.Name),
				Content: M{"application/json": M{
					"schema": M{
						"$ref": ref,
					},
				}},
			}
		}
		if len(endpoint.Tags) > 0 {
			tags = endpoint.Tags
		}

		method := &DocMethodInfo{
			Summary:    summary,
			Parameters: parameters,
			Responses:  resp,
			Tags:       tags,
			Security:   sec,
		}

		if endpoint.requestbody != nil {
			if endpoint.IsPost {

				key := fmt.Sprintf("Request%s", kk)

				gd.schemas[key] = M{
					"type":       "object",
					"properties": gd.DocTagsCustom(endpoint.requestbody, false),
				}
				ref := fmt.Sprintf("#/components/schemas/%s", key)

				method.RequestBody = &DocResponse{
					Description: fmt.Sprintf("%s model", endpoint.Name),
					Content: M{"application/json": M{
						"schema": M{
							"$ref": ref,
						},
					}},
				}
			} else {
				data := gd.DocTagsCustom(endpoint.requestbody, false)
				for key, val := range data {
					parameters = append(parameters, &DocParameter{
						Name:     key,
						In:       "query",
						Required: false,
						Schema: struct {
							Type    string "json:\"type,omitempty\""
							Maximum int    "json:\"maximum,omitempty\""
							Format  string "json:\"format,omitempty\""
						}{
							Type: val.(M)["type"].(string),
						},
					})
				}
				method.Parameters = parameters
			}
		}

		var endpointItem DocEndPoint

		if doc, ok := gd.paths[endpoint.docpath].(DocEndPoint); ok {
			if endpoint.IsPost {
				doc.Post = method
			} else {
				doc.Get = method
			}
			endpointItem = doc
		} else {
			endpointItem = DocEndPoint{}
			if endpoint.IsPost {
				endpointItem.Post = method
			} else {
				endpointItem.Get = method
			}
		}
		gd.paths[endpoint.docpath] = endpointItem
	}
}
func (gd *GenerateDoc) Generate() {
	gd.schemas = M{}
	gd.paths = M{}

	for _, model := range gd.app.models {
		for _, endpoint := range model.GetEndPoints() {
			gd.GenerateDocItem(model, endpoint, false, false, false)
		}
		for _, endpoint := range model.PostEndPoints() {
			gd.GenerateDocItem(model, endpoint, true, false, false)
		}
		for _, endpoint := range model.PutEndPoints() {
			gd.GenerateDocItem(model, endpoint, false, true, false)
		}
		for _, endpoint := range model.DeleteEndPoints() {
			gd.GenerateDocItem(model, endpoint, false, false, true)
		}

	}
	gd.GenerateOtherEndpoints()
	gd.schemas["NotFound"] = M{
		"type": "object",
		"properties": M{
			"message": M{
				"type": "string",
			},
			"status_code": M{
				"type": "integer",
			},
		},
	}
	gd.schemas["Unauthorized"] = M{
		"type": "object",
		"properties": M{
			"message": M{
				"type": "string",
			},
			"error": M{
				"type": "string",
			},
			"status_code": M{
				"type": "integer",
			},
		},
	}
	gd.schemas["ServerError"] = M{
		"type": "object",
		"properties": M{
			"message": M{
				"type": "string",
			},
			"error": M{
				"type": "object",
			},
			"status_code": M{
				"type": "integer",
			},
		},
	}
	var servers []M
	for _, uri := range gd.app.BaseURL {
		servers = append(servers, M{"url": uri})
	}
	data := M{
		"paths": gd.paths,
		"components": M{
			"schemas": gd.schemas,
			/*"securitySchemes": M{
				"bearerAuth": M{
					"type":         "http",
					"scheme":       "bearer",
					"bearerFormat": "JWT",
				},
			},*/
		},
		/*"security": []M{
			M{"bearerAuth": []M{}},
		},*/
		"openapi": "3.0.3",
		"servers": servers,
		"info": DocInfo{
			Version: "1.0",
			Title:   gd.app.Name,
		},
	}
	gd.data = data

}
func (gd *GenerateDoc) ResponseUI(c *fiber.Ctx) error {
	uiText := `
	<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml">
<head>
    <meta charset="UTF-8">
    <title>API DOC</title>
    <link rel="stylesheet" type="text/css" href="https://unpkg.com/swagger-ui-dist@3.12.1/swagger-ui.css">


</head>
<body>

<div id="swagger-ui"></div>

<script src="https://unpkg.com/swagger-ui-dist@3.12.1/swagger-ui-standalone-preset.js"></script>
<script src="https://unpkg.com/swagger-ui-dist@3.12.1/swagger-ui-bundle.js"></script>

<script>

    window.onload = function() {
        // Build a system
        const ui = SwaggerUIBundle({
            url: "/doc.json",
            dom_id: '#swagger-ui',
            deepLinking: false,
            presets: [
                SwaggerUIBundle.presets.apis,
                SwaggerUIStandalonePreset
            ],
            plugins: [
				SwaggerUIBundle.plugins.DownloadUrl
            ],
            layout: "StandaloneLayout",
        })
        window.ui = ui
    }
</script>
</body>
</html>`
	c.Set("content-type", "text/html")
	c.Response().BodyWriter().Write([]byte(uiText))
	return nil
}
func (gd *GenerateDoc) Response(c *fiber.Ctx) error {
	return c.JSON(gd.data)
}
func NewDoc(app *App) *GenerateDoc {
	doc := &GenerateDoc{
		app: app,
	}

	doc.Generate()
	app.RegisterGetEndpoint("/doc.json", true, nil, nil, doc.Response, nil)
	app.RegisterGetEndpoint("/doc/", true, nil, nil, doc.ResponseUI, nil)
	return doc
}
