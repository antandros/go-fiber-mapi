package main

import (
	"fmt"
	"time"

	gofibermapi "github.com/antandros/go-fiber-mapi"
	"github.com/antandros/go-fiber-mapi/app"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/crypto/bcrypt"
)

type PriceTimes struct {
	Company Company   `json:"company,omitempty"`
	Zumpany []Company `json:"zumpany,omitempty"`
}

type PriceTimesXN struct {
	Source  string
	Ticker  string
	Price   float64
	Company primitive.ObjectID
	Time    time.Time
}

type Company struct {
	Name string `json:"name,omitempty"`
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

type PriceQuery struct {
	StartDate string `json:"start_date,omitempty"`
	EndtDate  string `json:"endt_date,omitempty"`
	Ticker    string `json:"ticker,omitempty"`
	Fiat      string `json:"fiat,omitempty"`
	Limit     int    `json:"limit,omitempty"`
	Offset    int    `json:"offset,omitempty"`
	Aggregate string `json:"aggregate,omitempty"`
}

func main() {
	dapp := gofibermapi.NewApp("mongodb://root:example@10.4.0.102:27017/", "test_fiber_api", "./")

	dapp.BaseURL = "http://localhost:8766"
	prices2 := app.NewModel[PriceTimes]("price_times2")
	prices2.SoftDelete = true
	prices2.QueryParams = PriceQuery{}

	dapp.RegisterModel(prices2)
	dapp.Run(":8766")
}
