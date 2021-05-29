package main

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/srinathgs/mysqlstore"
	"golang.org/x/crypto/bcrypt"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

// 都市情報API
type City struct {
	ID          int    `json:"id,omitempty" db:"ID"`
	Name        string `json:"name,omitempty" db:"Name"`
	CountryCode string `json:"countryCode,omitempty" db:"CountryCode"`
	District    string `json:"district,omitempty" db:"District"`
	Population  int    `json:"population,omitempty" db:"Population"`
}
type CityPopulation struct {
	ID         int `json:"id,omitempty" db:"ID"`
	Population int `json:"population,omitempty" db:"Population"`
}

// ログイン周り
type LoginRequestBody struct {
	Username string `json:"username,omitempty" form:"username"`
	Password string `json:"password,omitempty" form:"password"`
}
type User struct {
	Username   string `json:"username,omitempty" db:"Username"`
	HashedPass string `json:"-" db:"HashedPass"`
}

var (
	db *sqlx.DB
)

func main() {
	_db, err := sqlx.Connect("mysql", fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8&parseTime=True&loc=Local", os.Getenv("DB_USERNAME"), os.Getenv("DB_PASSWORD"), os.Getenv("DB_HOSTNAME"), os.Getenv("DB_PORT"), os.Getenv("DB_DATABASE")))
	if err != nil {
		log.Fatalf("Cannot Connect to Database: %s", err)
	}
	db = _db

	store, err := mysqlstore.NewMySQLStoreFromConnection(db.DB, "sessions", "/", 60*60*24*14, []byte("secret-token"))
	if err != nil {
		panic(err)
	}

	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(session.Middleware(store))

	e.GET("/ping", func(c echo.Context) error {
		return c.String(http.StatusOK, "pong")
	})
	e.POST("/login", postLoginHandler)
	e.POST("/signup", postSignUpHandler)

	withLogin := e.Group("")
	withLogin.Use(checkLogin)
	withLogin.GET("/cities/:cityName", getCityInfoHandler)
	withLogin.POST("/city", postNewCityHandler)
	withLogin.PATCH("/city/population", updateCityPopulationHandler)

	e.Start(":" + os.Getenv("API_PORT"))
}

func postSignUpHandler(c echo.Context) error {
	req := LoginRequestBody{}
	c.Bind(&req)

	// TODO:もう少し細かくする
	if req.Password == "" || req.Username == "" {
		return c.String(http.StatusBadRequest, "項目が空です")
	}

	hashedPass, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("bcrypt generate error: %v", err))
	}

	// ユーザーの存在チェック
	var count int

	err = db.Get(&count, "SELECT COUNT(*) FROM users WHERE Username=?", req.Username)
	if err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("db error: %v", err))
	}

	if count > 0 {
		return c.String(http.StatusConflict, "ユーザーがすでに存在しています")
	}

	_, err = db.Exec("INSERT INTO users (Username, HashedPass) VALUES (?, ?)", req.Username, hashedPass)
	if err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("db error: %v", err))
	}
	return c.NoContent(http.StatusCreated)
}

func postLoginHandler(c echo.Context) error {
	req := LoginRequestBody{}
	c.Bind(&req)

	fmt.Println(req.Username)
	user := User{}
	err := db.Get(&user, "SELECT * FROM users WHERE username=?", req.Username)
	if err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("db error: %v", err))
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.HashedPass), []byte(req.Password))
	if err != nil {
		if err == bcrypt.ErrMismatchedHashAndPassword {
			return c.NoContent(http.StatusForbidden)
		} else {
			return c.NoContent(http.StatusInternalServerError)
		}
	}

	sess, err := session.Get("sessions", c)
	if err != nil {
		fmt.Println(err)
		return c.String(http.StatusInternalServerError, "something wrong in getting session")
	}
	sess.Values["userName"] = req.Username
	sess.Save(c.Request(), c.Response())

	return c.NoContent(http.StatusOK)
}

func checkLogin(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		sess, err := session.Get("sessions", c)
		if err != nil {
			fmt.Println(err)
			return c.String(http.StatusInternalServerError, "something wrong in getting session")
		}

		if sess.Values["userName"] == nil {
			return c.String(http.StatusForbidden, "please login")
		}
		c.Set("userName", sess.Values["userName"].(string))
		return next(c)
	}
}

func getCityInfoHandler(c echo.Context) error {
	cityName := c.Param("cityName")
	fmt.Println(cityName)

	var city City
	if err := db.Get(&city, "SELECT * FROM city WHERE Name=?", cityName); errors.Is(err, sql.ErrNoRows) {
		log.Printf("No Such City Name=%s", cityName)
	} else if err != nil {
		log.Fatalf("DB Error: %s", err)
	}
	return c.JSON(http.StatusOK, city)
}

func postNewCityHandler(c echo.Context) error {
	city := new(City)
	err := c.Bind(city)

	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Bad Request")
	}
	result, err := db.Exec("INSERT INTO city (Name, CountryCode, District, Population) VALUES(?,?,?,?)", city.Name, city.CountryCode, city.District, city.Population)
	if err != nil {
		fmt.Printf("DB error: %s\n", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "DB error")
	}
	fmt.Println(result)
	return c.String(http.StatusOK, "OK")
}

func updateCityPopulationHandler(c echo.Context) error {
	cityPopulation := new(CityPopulation)
	err := c.Bind(cityPopulation)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Bad Request")
	}

	result, err := db.Exec("UPDATE city SET population=? where ID=?", cityPopulation.Population, cityPopulation.ID)
	if err != nil {
		fmt.Printf("DB error: %s\n", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "DB error")
	}
	fmt.Println(result)
	return c.String(http.StatusOK, "OK")
}
