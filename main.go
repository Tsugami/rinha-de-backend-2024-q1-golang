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
	Total int `json:"saldo"`
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

type RinhaError struct {
	Message    string `json:"message"`
	StatusCode int    `json:"status_code"`
	Error      error
}

func NewRinhaServerError(err error) *RinhaError {
	return &RinhaError{Error: err, Message: "Internal Server Error", StatusCode: 500}
}

func NewRinhaError(error error, message string, statusCode int) *RinhaError {
	return &RinhaError{Message: message, StatusCode: statusCode}
}

func createTransaction(conn *pgx.Conn, id string, newTransaction CreateTransactionInput) (*Balance, *RinhaError) {
	tx, err := conn.Begin(context.Background())
	if err != nil {
		return nil, NewRinhaServerError(err)
	}

	defer tx.Rollback(context.Background())
	balance := &Balance{}

	err = tx.QueryRow(
		context.Background(),
		"UPDATE accounts SET saldo = saldo - $1 WHERE id = $2 AND (saldo - $1) > ~limite RETURNING limite, saldo",
		newTransaction.Value,
		id).Scan(&balance.Limit, &balance.Total)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, NewRinhaError(err, "Saldo insuficiente", http.StatusUnprocessableEntity)
		}

		return nil, NewRinhaServerError(err)
	}

	_, err = tx.Exec(context.Background(), "INSERT INTO transactions (valor, tipo, descricao, account_id) VALUES ($1, $2, $3, $4)", newTransaction.Value, newTransaction.Type, newTransaction.Description, id)
	if err != nil {
		return nil, NewRinhaServerError(err)
	}

	err = tx.Commit(context.Background())
	if err != nil {
		return nil, NewRinhaServerError(err)
	}

	return balance, nil
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

		balance := ExtractBalance{Date: time.Now()}
		lastTransactions := []ExtractTransaction{}

		err := conn.QueryRow(context.Background(), "SELECT saldo, limite FROM accounts WHERE id = $1", params.ID).Scan(&balance.Total, &balance.Limit)
		if err != nil {
			fmt.Println("query", err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "Internal Server Error", "context": "query"})
			return
		}

		rows, err := conn.Query(context.Background(), "SELECT valor, tipo, descricao, realizada_em FROM transactions WHERE account_id = $1 ORDER BY realizada_em DESC LIMIT 10", params.ID)
		if err != nil {
			fmt.Println("query", err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "Internal Server Error", "context": "query"})
			return
		}

		defer rows.Close()

		for rows.Next() {
			var transaction ExtractTransaction
			err = rows.Scan(&transaction.Value, &transaction.Type, &transaction.Description, &transaction.Date)
			if err != nil {
				fmt.Println("scan", err)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "Internal Server Error", "context": "scan"})
			}
		}

		extract := Extract{
			Balance:          balance,
			LastTransactions: lastTransactions,
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

		balance, err := createTransaction(conn, id, newTransaction)
		if err != nil {
			fmt.Println("createTransaction", err)
			c.AbortWithStatusJSON(err.StatusCode, gin.H{"message": err.Message})
			return
		}

		c.JSON(http.StatusOK, balance)
	})

	return r
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func main() {
	databaseUrl := getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/postgres")
	conn, err := pgx.Connect(context.Background(), databaseUrl)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}

	defer conn.Close(context.Background())

	r := setupRouter(conn)
	r.Run()
}
