package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

var db = make(map[string]string)

type RinhaParams struct {
	ID int `uri:"id" binding:"required"`
}

type Balance struct {
	Total int `json:"total"`
	Limit int `json:"limite"`
}

type CreateTransactionInput struct {
	Value       int    `json:"valor" binding:"required"`
	Type        string `json:"tipo" binding:"required"`
	Description string `json:"descricao" binding:"required"`
}

type ExtractTransaction struct {
	Value       int       `json:"valor"`
	Type        string    `json:"tipo"`
	Description string    `json:"descricao"`
	Date        time.Time `json:"realizada_em"`
}

type ExtractBalance struct {
	Total int       `json:"total"`
	Limit int       `json:"limite"`
	Date  time.Time `json:"data_extrato"`
}

type Extract struct {
	Balance          ExtractBalance       `json:"saldo"`
	LastTransactions []ExtractTransaction `json:"ultimas_transacoes"`
}

func setupRouter(conn *pgx.Conn) *gin.Engine {
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
			Balance: ExtractBalance{
				Total: 100,
				Limit: 1000,
				Date:  time.Now(),
			},
			LastTransactions: []ExtractTransaction{
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
		id := c.Param("id")

		newTransaction := CreateTransactionInput{}

		if err := c.ShouldBindJSON(&newTransaction); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest,
				gin.H{
					"message": "Invalid inputs. Please check your inputs",
				})
			return
		}

		fmt.Println(id, newTransaction)

		tx, err := conn.Begin(context.Background())
		if err != nil {
			fmt.Println("begin", err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "Internal Server Error", "context": "begin"})
			return
		}

		defer tx.Rollback(context.Background())

		balance := Balance{}

		err = tx.QueryRow(
			context.Background(),
			"UPDATE accounts SET saldo = saldo - $1 WHERE id = $2 AND (saldo - $1) > ~limite RETURNING limite, saldo",
			newTransaction.Value,
			id).Scan(&balance.Limit, &balance.Total)
		if err != nil {
			if err.Error() == "no rows in result set" {
				c.AbortWithStatusJSON(http.StatusUnprocessableEntity, gin.H{"message": "Saldo insuficiente"})
				return
			}

			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "Internal Server Error", "context": "new-balance"})
			return
		}

		_, err = tx.Exec(context.Background(), "INSERT INTO transactions (valor, tipo, descricao, account_id) VALUES ($1, $2, $3, $4)", newTransaction.Value, newTransaction.Type, newTransaction.Description, id)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "Internal Server Error", "context": "new-transaction"})
			return
		}

		err = tx.Commit(context.Background())
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "Internal Server Error", "context": "commit"})
			return
		}

		c.JSON(http.StatusOK, balance)
	})

	return r
}

func main() {
	databaseUrl := "postgres://postgres:postgres@localhost:5432/postgres"
	conn, err := pgx.Connect(context.Background(), databaseUrl)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}

	defer conn.Close(context.Background())

	r := setupRouter(conn)
	r.Run(":9999")
}
