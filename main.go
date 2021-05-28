package main

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

type City struct {
	ID          int    `json:"id,omitempty" db:"ID"`
	Name        string `json:"name,omitempty" db:"Name"`
	CountryCode string `json:"countryCode,omitempty" db:"CountryCode"`
	District    string `json:"district,omitempty" db:"District"`
	Population  int    `json:"population,omitempty" db:"Population"`
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

	e := echo.New()
	e.GET("/cities/:cityName", getCityInfoHandler)
	e.POST("city", postNewCityHandler)
	e.Start(":" + os.Getenv("API_PORT"))
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
