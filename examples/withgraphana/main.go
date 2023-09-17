package main

import (
	"errors"
	"fmt"
	"strings"
	"time"

	gofibermapi "github.com/antandros/go-fiber-mapi"
	"github.com/antandros/go-fiber-mapi/app"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/crypto/bcrypt"
)

type PriceTimes struct {
	Source    string
	Ticker    string
	Price     float64
	Time      time.Time
	CompanyId primitive.ObjectID
}

type PriceTimesXN struct {
	Source  string
	Ticker  string
	Price   float64
	Company primitive.ObjectID
	Time    time.Time
}

type RequestParamsItem struct {
	Ticker string `json:"ticker,omitempty"`
	Source string `json:"source,omitempty"`
}
type ResponseAggr struct {
	Id  string `bson:"_id" json:"ticker"`
	Avg uint64 `bson:"avg" json:"avg,omitempty"`
}

type Company struct {
	Name string
}
type Login struct {
	Mail     string `json:"mail,omitempty"`
	Password string `json:"password,omitempty"`
}
type LoginResponse struct {
	AccessToken string `json:"access_token,omitempty"`
	Error       string `json:"error,omitempty"`
}
type User struct {
	Mail     string
	Password string
	Company  primitive.ObjectID
	Id       primitive.ObjectID
}

func loginFunc(dapp *app.App, c *fiber.Ctx) error {
	reqBody := new(Login)
	err := c.BodyParser(reqBody)
	if err != err {
		panic(err)
	}
	resp := dapp.FindOneCollection("user", app.M{"mail": reqBody.Mail})
	if resp.Err() != nil {
		return c.JSON(LoginResponse{
			Error: "mail or password didnt match",
		})
	}
	var user User
	err = resp.Decode(&user)
	if err != nil {
		panic(err)
	}
	hmacSampleSecret := []byte("AllYourBase")
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(reqBody.Password))
	if err == nil {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"iss": user.Id.Hex(),
			"sub": user.Company.Hex(),
			"nbf": time.Now().Unix(),
			"exp": time.Now().Add(time.Hour * 1).Unix(),
		})
		tokenString, err := token.SignedString(hmacSampleSecret)
		if err != nil {
			fmt.Println(err)
			panic(err)
		}
		return c.JSON(LoginResponse{
			AccessToken: tokenString,
		})
	} else {
		return c.JSON(LoginResponse{
			Error: "mail or password didnt match",
		})
	}
}
func main() {
	dapp := gofibermapi.NewApp("mongodb://root:example@10.4.0.102:27017/", "test_fiber_api", "./log/")
	dapp.Description = "Deneme felan"
	dapp.Name = "Test api"
	dapp.SaveLog = true
	dapp.Debug = true
	dapp.BaseURL = "http://127.0.0.1:8766/"
	prices := app.NewModel[PriceTimes]("price_times")
	prices.SoftDelete = true
	prices.UpdateOnAdd(func(item app.M, c *fiber.Ctx) (app.M, error) {
		cid := c.Locals("cid")
		if cid != nil {
			item["company_id"] = cid.(primitive.ObjectID)
			return item, nil
		} else {
			return nil, errors.New("cid not found")
		}
	})
	/*prices.UpdateOnUpdate(func(item app.M, c *fiber.Ctx) (app.M, error) {
		return nil, nil
	})*/
	prices2 := app.NewModel[PriceTimesXN]("price_times2")
	prices2.SoftDelete = true
	query := app.M{
		"$group":
		/**
		 * _id: The id of the group.
		 * fieldN: The first field name.
		 */
		app.M{
			"_id": "$ticker",
			"avg": app.M{
				"$avg": "$price",
			},
		},
	}

	prices.AddAggrageEndPoint("test3", "get", ResponseAggr{}, RequestParamsItem{}, []app.M{

		app.M{"$match": app.M{
			"ticker": "{{ .Ticker }}",
		}},
		query,
	})
	dapp.RegisterModel(prices2)
	dapp.RegisterModel(prices)
	endpoint := dapp.RegisterPostEndpoint("/login", true, Login{}, LoginResponse{}, func(c *fiber.Ctx) error {
		return loginFunc(dapp, c)
	})
	endpoint.Name = "Login Endpoint"
	endpoint.Description = "Login Endpoint"
	dapp.SetAuthMiddleware(func(c *fiber.Ctx) (app.M, error) {
		var tokenString string
		tokenData := c.Get("authorization", "")
		if tokenData == "" {
			return nil, errors.New("authorization header required")
		}
		tokeItem := strings.Split(tokenData, " ")
		if !strings.EqualFold(tokeItem[0], "Bearer") {
			return nil, errors.New("authorization header must be bearer")
		}
		tokenString = tokeItem[1]
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return []byte("AllYourBase"), nil
		})
		if err != nil {

			if errors.Is(err, jwt.ErrTokenMalformed) {
				return app.M{}, errors.New("That's not even a token")
			} else if errors.Is(err, jwt.ErrTokenSignatureInvalid) {
				// Invalid signature
				return app.M{}, errors.New("Invalid signature")
			} else if errors.Is(err, jwt.ErrTokenExpired) || errors.Is(err, jwt.ErrTokenNotValidYet) {
				// Token is either expired or not active yet
				fmt.Println("ErrTokenExpired", errors.Is(err, jwt.ErrTokenExpired))
				return app.M{}, errors.New("Timing is everything")
			} else {
				return app.M{}, errors.New(fmt.Sprintf("Couldn't handle this token: %v", err))
			}
		} else {
			subj, _ := token.Claims.GetSubject()
			uid, _ := token.Claims.GetIssuer()
			cid, _ := primitive.ObjectIDFromHex(subj)
			c.Locals("uid", uid)
			c.Locals("sub", subj)
			c.Locals("cid", cid)
			return app.M{"company_id": cid}, nil
		}

	})
	dapp.Run(":8766")
}
