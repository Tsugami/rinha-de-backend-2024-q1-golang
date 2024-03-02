package main

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

var db = make(map[string]string)

type RinhaParams struct {
	ID int `uri:"id" binding:"required"`
}

type Balance struct {
	Total int       `json:"total"`
	Limit int       `json:"limite"`
	Date  time.Time `json:"data_extrato"`
}

type Transaction struct {
	Value       int       `json:"valor"`
	Type        string    `json:"tipo"`
	Description string    `json:"descricao"`
	Date        time.Time `json:"realizada_em"`
}

type Extract struct {
	Balance          Balance       `json:"saldo"`
	LastTransactions []Transaction `json:"ultimas_transacoes"`
}

func setupRouter() *gin.Engine {
	r := gin.Default()

	r.GET("/clientes/:id/extrato", func(c *gin.Context) {
		var params RinhaParams

		if err := c.ShouldBindUri(&params); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		if params.ID < 1 || params.ID > 5 {
			c.Status(http.StatusNotFound)
			return
		}

		extract := Extract{
			Balance: Balance{
				Total: 100,
				Limit: 1000,
				Date:  time.Now(),
			},
			LastTransactions: []Transaction{
				{
					Value:       100,
					Type:        "debito",
					Description: "Compra de um produto",
					Date:        time.Now(),
				},
			},
		}

		c.JSON(http.StatusOK, extract)
	})

	r.POST("/clientes/:id/transacoes", func(c *gin.Context) {
		newTransaction := Transaction{
			Value:       100,
			Type:        "debito",
			Description: "Compra de um produto",
			Date:        time.Now(),
		}

		c.JSON(http.StatusOK, newTransaction)
	})

	return r
}

func main() {
	r := setupRouter()
	r.Run(":9999")
}
